package settings

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type memStore struct{ rows map[string]Row }

func (m *memStore) LoadSettings(context.Context) ([]Row, error) {
	var out []Row
	for _, r := range m.rows {
		out = append(out, r)
	}
	return out, nil
}
func (m *memStore) SaveSetting(_ context.Context, r Row) error { m.rows[r.Key] = r; return nil }
func (m *memStore) DeleteSetting(_ context.Context, k string) error {
	delete(m.rows, k)
	return nil
}

func newSvc(t *testing.T, env map[string]string, secretKey string) (*Service, *memStore) {
	t.Helper()
	c, err := NewCipher(secretKey)
	if err != nil {
		t.Fatal(err)
	}
	st := &memStore{rows: map[string]Row{}}
	s := NewService(st, c, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.env = func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	if err := s.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s, st
}

const key16 = "uma-chave-mestra-de-teste"

func TestPrecedence_DBThenEnvThenDefault(t *testing.T) {
	s, _ := newSvc(t, map[string]string{"DIGEST_HOUR": "7"}, key16)
	if got := s.Int("DIGEST_HOUR"); got != 7 {
		t.Errorf("ambiente: %d", got)
	}
	if got := s.Int("SYNC_INTERVAL_HOURS"); got != 6 {
		t.Errorf("padrão: %d", got)
	}
	if _, err := s.Set(context.Background(), map[string]string{"DIGEST_HOUR": "20"}); err != nil {
		t.Fatal(err)
	}
	if got := s.Int("DIGEST_HOUR"); got != 20 {
		t.Errorf("banco vence o ambiente: %d", got)
	}
	f := fieldOf(t, s, "DIGEST_HOUR")
	if f.Source != SourceDB {
		t.Errorf("origem: %s", f.Source)
	}
	if err := s.Reset(context.Background(), "DIGEST_HOUR"); err != nil || s.Int("DIGEST_HOUR") != 7 {
		t.Errorf("reset volta ao ambiente: %v %d", err, s.Int("DIGEST_HOUR"))
	}
}

func fieldOf(t *testing.T, s *Service, key string) Field {
	t.Helper()
	for _, f := range s.Fields("telegram") {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("campo %s não listado", key)
	return Field{}
}

func TestSecrets_EncryptedAtRestAndNeverReturned(t *testing.T) {
	s, st := newSvc(t, nil, key16)
	if _, err := s.Set(context.Background(), map[string]string{"PLUGGY_CLIENT_SECRET": "super-segredo"}); err != nil {
		t.Fatal(err)
	}
	row := st.rows["PLUGGY_CLIENT_SECRET"]
	if !row.Secret || strings.Contains(row.Value, "super-segredo") {
		t.Fatalf("o banco não pode guardar o segredo em claro: %+v", row)
	}
	f := fieldOf(t, s, "PLUGGY_CLIENT_SECRET")
	if f.Value != "" || !f.IsSet {
		t.Errorf("segredo não volta para a tela, só 'configurado': %+v", f)
	}
	if s.Get("PLUGGY_CLIENT_SECRET") != "super-segredo" {
		t.Error("o app precisa do valor em claro")
	}

	// Outro app com a mesma chave lê; com outra chave, ignora o segredo.
	again, _ := newSvc(t, nil, key16)
	again.store = st
	again.Load(context.Background()) //nolint:errcheck
	if again.Get("PLUGGY_CLIENT_SECRET") != "super-segredo" {
		t.Error("deveria decifrar com a mesma chave")
	}
	other, _ := newSvc(t, nil, "outra-chave-mestra-qualquer")
	other.store = st
	other.Load(context.Background()) //nolint:errcheck
	if other.Get("PLUGGY_CLIENT_SECRET") != "" {
		t.Error("com outra chave o segredo deve ser ignorado")
	}
}

func TestSecrets_NeedEncryptionKey(t *testing.T) {
	s, _ := newSvc(t, nil, "")
	if s.EncryptionEnabled() {
		t.Fatal("sem chave mestra, sem segredos")
	}
	if _, err := s.Set(context.Background(), map[string]string{"GEMINI_API_KEY": "abc"}); err != ErrNoEncryptionKey {
		t.Errorf("err = %v", err)
	}
	if _, err := s.Set(context.Background(), map[string]string{"DIGEST_HOUR": "8"}); err != nil {
		t.Errorf("configuração comum não precisa da chave: %v", err)
	}
	if _, err := NewCipher("curta"); err == nil {
		t.Error("chave mestra curta deveria ser recusada")
	}
}

func TestCiphertext_BoundToKeyName(t *testing.T) {
	c, _ := NewCipher(key16)
	enc, _ := c.Encrypt("PLUGGY_CLIENT_SECRET", "x")
	if _, err := c.Decrypt("GEMINI_API_KEY", enc); err == nil {
		t.Error("segredo copiado para outra chave não pode decifrar")
	}
	if _, err := c.Decrypt("PLUGGY_CLIENT_SECRET", "%%%"); err == nil {
		t.Error("lixo deve dar erro")
	}
}

func TestValidation(t *testing.T) {
	s, st := newSvc(t, nil, key16)
	bad := map[string]string{
		"DIGEST_HOUR":         "24",
		"DIGEST_WEEKDAY":      "segunda",
		"DIGEST_ENABLED":      "talvez",
		"DIGEST_TIMEZONE":     "Marte/Olimpo",
		"SYNC_LOOKBACK_DAYS":  "400",
		"SYNC_INTERVAL_HOURS": "0",
		"PLUGGY_ITEM_IDS":     "nao-e-uuid",
		"OWN_NAMES":           "Ana",
		"TELEGRAM_CHAT_ID":    "-5",
		"EVOLUTION_API_URL":   "ftp://x",
		"OWNER_PHONE":         "55 11",
		"COACH_GEMINI_MODEL":  "com espaço",
		"NAO_EXISTE":          "x",
		"DIGEST_TIMEZONE\n":   "x",
	}
	for k, v := range bad {
		if _, err := s.Set(context.Background(), map[string]string{k: v}); err == nil {
			t.Errorf("%s=%q deveria ser recusado", k, v)
		}
	}
	if _, err := s.Set(context.Background(), map[string]string{"DIGEST_HOUR": "8", "SYNC_LOOKBACK_DAYS": "999"}); err == nil {
		t.Fatal("um valor inválido recusa o lote")
	}
	if len(st.rows) != 0 {
		t.Errorf("nada deve ser gravado quando o lote é recusado: %v", st.rows)
	}
	for k, v := range map[string]string{
		"DIGEST_TIMEZONE": "America/Sao_Paulo", "OWN_NAMES": "Maria da Silva, João Pereira",
		"PLUGGY_ITEM_IDS": "592c8fdb-a1b3-495c-b2d6-ec9027cf386d", "DIGEST_WEEKDAY": "friday", "TELEGRAM_CHAT_ID": "123456",
	} {
		if _, err := s.Set(context.Background(), map[string]string{k: v}); err != nil {
			t.Errorf("%s=%q: %v", k, v, err)
		}
	}
}

func TestPendingRestart(t *testing.T) {
	s, _ := newSvc(t, map[string]string{"TELEGRAM_CHAT_ID": "111"}, key16)
	if s.NeedsRestart("telegram") {
		t.Fatal("nada mudou ainda")
	}
	s.Set(context.Background(), map[string]string{"DIGEST_HOUR": "10"}) //nolint:errcheck
	if s.NeedsRestart("telegram") {
		t.Error("configuração viva não pede reinício")
	}
	s.Set(context.Background(), map[string]string{"TELEGRAM_CHAT_ID": "222"}) //nolint:errcheck
	if !s.NeedsRestart("telegram") || !fieldOf(t, s, "TELEGRAM_CHAT_ID").PendingRestart {
		t.Error("o token/ID do canal só vale depois de reiniciar")
	}
	s.Set(context.Background(), map[string]string{"TELEGRAM_CHAT_ID": "111"}) //nolint:errcheck
	if s.NeedsRestart("telegram") {
		t.Error("voltar ao valor do início cancela o pedido de reinício")
	}
}

func TestFields_OnlyActiveChannel(t *testing.T) {
	s, _ := newSvc(t, nil, key16)
	for _, f := range s.Fields("telegram") {
		if f.Channel == "whatsapp" {
			t.Errorf("campo do WhatsApp apareceu no Telegram: %s", f.Key)
		}
	}
	var hasTG bool
	for _, f := range s.Fields("whatsapp") {
		if f.Channel == "telegram" {
			t.Errorf("campo do Telegram apareceu no WhatsApp: %s", f.Key)
		}
		hasTG = hasTG || f.Key == "OWNER_PHONE"
	}
	if !hasTG {
		t.Error("campos do canal ativo deveriam aparecer")
	}
}

func TestTypedAccessors(t *testing.T) {
	s, _ := newSvc(t, map[string]string{
		"DIGEST_WEEKDAY": "sunday", "DIGEST_HOUR": "18", "DIGEST_TIMEZONE": "America/Sao_Paulo",
		"PLUGGY_CLIENT_ID": "id", "PLUGGY_CLIENT_SECRET": "sec", "PLUGGY_ITEM_IDS": "a, b ,", "OWN_NAMES": "Maria Silva , João Souza",
	}, key16)
	enabled, wd, hour, loc := s.Digest()
	if !enabled || wd != time.Sunday || hour != 18 || loc.String() != "America/Sao_Paulo" {
		t.Errorf("digest: %v %v %v %v", enabled, wd, hour, loc)
	}
	sy := s.Sync()
	if !sy.Configured() || len(sy.ItemIDs) != 2 || sy.Interval != 6*time.Hour || sy.LookbackDays != 60 {
		t.Errorf("sync: %+v", sy)
	}
	if names := s.List("OWN_NAMES"); len(names) != 2 || names[0] != "Maria Silva" {
		t.Errorf("nomes: %v", names)
	}
	empty, _ := newSvc(t, nil, key16)
	if empty.Sync().Configured() {
		t.Error("sem credenciais não está configurado")
	}
}

func TestOnChangeNotifies(t *testing.T) {
	s, _ := newSvc(t, nil, key16)
	n := 0
	s.OnChange(func() { n++ })
	s.Set(context.Background(), map[string]string{"DIGEST_HOUR": "5"}) //nolint:errcheck
	s.Reset(context.Background(), "DIGEST_HOUR")                       //nolint:errcheck
	if n != 2 {
		t.Errorf("notificações = %d", n)
	}
}

func TestSensitiveCoversSecretsAndAccessControl(t *testing.T) {
	for _, d := range Defs {
		if d.Kind == KindSecret && !d.Sensitive() {
			t.Errorf("segredo %s deve exigir a senha", d.Key)
		}
	}
	for _, k := range []string{"PLUGGY_ITEM_IDS", "TELEGRAM_CHAT_ID", "EVOLUTION_API_URL", "OWNER_PHONE", "ALLOWED_NUMBERS"} {
		if d, _ := Lookup(k); !d.Sensitive() {
			t.Errorf("%s redireciona dados ou abre acesso: deve exigir a senha", k)
		}
	}
	for _, k := range []string{"DIGEST_HOUR", "SYNC_INTERVAL_HOURS", "OWN_NAMES", "GEMINI_PAID_PLAN", "COACH_GEMINI_MODEL"} {
		if d, _ := Lookup(k); d.Sensitive() {
			t.Errorf("%s é ajuste comum: não deve pedir a senha", k)
		}
	}
}

// O canal só é gravado pelo setup: a página não mostra, não altera nem restaura.
func TestInternal_ChannelOnlyBySetup(t *testing.T) {
	s, st := newSvc(t, nil, key16)
	for _, f := range s.Fields("telegram") {
		if f.Key == "CHANNEL" {
			t.Fatal("CHANNEL apareceu na página")
		}
	}
	if _, err := s.Set(context.Background(), map[string]string{"CHANNEL": "telegram"}); err == nil {
		t.Fatal("a página não pode trocar o canal")
	}
	if err := s.Reset(context.Background(), "CHANNEL"); err == nil {
		t.Fatal("a página não pode restaurar o canal")
	}
	st.rows["CHANNEL"] = Row{Key: "CHANNEL", Value: "telegram"}
	if err := s.Load(context.Background()); err != nil || s.Get("CHANNEL") != "telegram" {
		t.Fatalf("o canal salvo pelo setup vale: %q %v", s.Get("CHANNEL"), err)
	}
}

// Adopt registra o que o setup já ligou: vale na hora, sem pedir reinício, e avisa quem escuta.
func TestAdopt_NoRestartAndNotifies(t *testing.T) {
	s, _ := newSvc(t, nil, key16)
	called := 0
	s.OnChange(func() { called++ })
	s.Adopt(map[string]string{"CHANNEL": "telegram", "TELEGRAM_BOT_TOKEN": "1:a", "TELEGRAM_CHAT_ID": "42"})
	if s.Get("TELEGRAM_CHAT_ID") != "42" || s.NeedsRestart("telegram") || called != 1 {
		t.Fatalf("adopt: id=%q restart=%v called=%d", s.Get("TELEGRAM_CHAT_ID"), s.NeedsRestart("telegram"), called)
	}
}
