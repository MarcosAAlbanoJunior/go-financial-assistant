package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/pluggy"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
	"github.com/google/uuid"
)

type memSettings struct{ rows map[string]settings.Row }

func (m *memSettings) LoadSettings(context.Context) ([]settings.Row, error) {
	var out []settings.Row
	for _, r := range m.rows {
		out = append(out, r)
	}
	return out, nil
}
func (m *memSettings) SaveSetting(_ context.Context, r settings.Row) error {
	m.rows[r.Key] = r
	return nil
}
func (m *memSettings) DeleteSetting(_ context.Context, k string) error { delete(m.rows, k); return nil }

type memAudit struct {
	mu      sync.Mutex
	entries []settings.AuditEntry
}

func (m *memAudit) RecordAudit(_ context.Context, e settings.AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e.At = time.Now()
	m.entries = append([]settings.AuditEntry{e}, m.entries...)
	return nil
}
func (m *memAudit) RecentAudit(_ context.Context, limit int) ([]settings.AuditEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.entries[:min(limit, len(m.entries))], nil
}

type fakeCleaner struct {
	cands     []domain.TransferCandidate
	cancelled []uuid.UUID
}

func (f *fakeCleaner) OwnTransferCandidates(context.Context) ([]domain.TransferCandidate, error) {
	return f.cands, nil
}
func (f *fakeCleaner) CancelPayments(_ context.Context, ids []uuid.UUID) (int64, error) {
	f.cancelled = append(f.cancelled, ids...)
	return int64(len(ids)), nil
}

type settingsEnv struct {
	store   *memSettings
	svc     *settings.Service
	cleaner *fakeCleaner
	restart atomic.Int32
	deps    *SettingsDeps
	audit   *memAudit
	notices []string
}

func newSettingsAPI(t *testing.T, secretKey string) (*settingsEnv, *Server) {
	t.Helper()
	return newSettingsAPIWith(t, secretKey, nil)
}

func newSettingsAPIWith(t *testing.T, secretKey string, factor *SecondFactor) (*settingsEnv, *Server) {
	t.Helper()
	cipher, err := settings.NewCipher(secretKey)
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := &memSettings{rows: map[string]settings.Row{}}
	svc := settings.NewService(store, cipher, logger)
	svc.Load(context.Background()) //nolint:errcheck

	e := &settingsEnv{store: store, svc: svc, cleaner: &fakeCleaner{}, audit: &memAudit{}}
	e.deps = &SettingsDeps{
		Service: svc, Channel: "telegram", Cleaner: e.cleaner,
		PluggyCheck: func(_ context.Context, id, secret string, items []string) ([]pluggy.ItemCheck, error) {
			if secret == "ruim" {
				return nil, errors.New("pluggy auth: HTTP 401: segredo-vazado-aqui")
			}
			out := make([]pluggy.ItemCheck, len(items))
			for i, it := range items {
				out[i] = pluggy.ItemCheck{ItemID: it, OK: it != "00000000-0000-0000-0000-000000000000"}
			}
			return out, nil
		},
		TelegramPing: func(_ context.Context, token string) (string, error) {
			if token == "ruim" {
				return "", errors.New("telegram getMe: 401 token-vazado")
			}
			return "meu_bot", nil
		},
		GeminiPing: func(_ context.Context, key string) error {
			if key == "ruim" {
				return errors.New("API key not valid chave-vazada")
			}
			return nil
		},
		Restart: func() { e.restart.Add(1) },
		Audit:   e.audit,
		Notify:  func(_ context.Context, text string) { e.notices = append(e.notices, text) },
	}
	s := NewServer(0, logger)
	s.SetSettings(e.deps)
	s.SetSecondFactor(factor)
	if err := s.MountAPI(StaticPassword(testPassword), &fakeReader{}); err != nil {
		t.Fatal(err)
	}
	return e, s
}

const goodKey = "uma-chave-mestra-de-teste"

func TestSettingsAPI_RequiresSession(t *testing.T) {
	_, s := newSettingsAPI(t, goodKey)
	for _, p := range [][2]string{{"GET", "/api/settings"}, {"PUT", "/api/settings"}, {"POST", "/api/settings/reset/DIGEST_HOUR"}, {"GET", "/api/settings/audit"},
		{"POST", "/api/settings/own-transfers/apply"}, {"POST", "/api/settings/test/pluggy"}, {"POST", "/api/restart"}} {
		if rec := do(s, p[0], p[1], "{}", jsonHdr); rec.Code != 401 {
			t.Errorf("%s %s sem sessão = %d", p[0], p[1], rec.Code)
		}
	}
}

func TestSettingsAPI_GetNeverReturnsSecrets(t *testing.T) {
	e, s := newSettingsAPI(t, goodKey)
	c := login(t, s)
	e.svc.Set(context.Background(), map[string]string{"PLUGGY_CLIENT_SECRET": "super-segredo", "GEMINI_API_KEY": "chave-gemini", "DIGEST_HOUR": "7"}) //nolint:errcheck

	rec := do(s, "GET", "/api/settings", "", nil, c)
	body := rec.Body.String()
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	if strings.Contains(body, "super-segredo") || strings.Contains(body, "chave-gemini") {
		t.Fatal("segredo vazou na resposta")
	}
	for _, want := range []string{`"key":"PLUGGY_CLIENT_SECRET"`, `"isSet":true`, `"source":"db"`, `"encryption":true`, `"channel":"telegram"`, `"title":"Telegram"`} {
		if !strings.Contains(body, want) {
			t.Errorf("faltou %s", want)
		}
	}
	if strings.Contains(body, "EVOLUTION_API_KEY") {
		t.Error("campos do WhatsApp não aparecem no canal Telegram")
	}
}

func TestSettingsAPI_PutValidationAndSecrets(t *testing.T) {
	e, s := newSettingsAPI(t, goodKey)
	c := login(t, s)

	for _, body := range []string{
		`{"values":{"DIGEST_HOUR":"30"}}`, `{"values":{"NAO_EXISTE":"x"}}`, `{"values":{}}`, `nao-json`,
		`{"values":{"DIGEST_HOUR":"8","PLUGGY_ITEM_IDS":"lixo"},"password":"` + testPassword + `"}`,
	} {
		if rec := do(s, "PUT", "/api/settings", body, jsonHdr, c); rec.Code != 400 {
			t.Errorf("%s = %d, quer 400: %s", body, rec.Code, rec.Body)
		}
	}
	if len(e.store.rows) != 0 {
		t.Errorf("lote recusado não grava nada: %v", e.store.rows)
	}

	rec := do(s, "PUT", "/api/settings", `{"values":{"DIGEST_HOUR":"8","PLUGGY_CLIENT_SECRET":"abc123"},"password":"`+testPassword+`"}`, jsonHdr, c)
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "abc123") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if e.svc.Int("DIGEST_HOUR") != 8 || e.svc.Get("PLUGGY_CLIENT_SECRET") != "abc123" {
		t.Error("valores deveriam valer na hora")
	}
	if strings.Contains(e.store.rows["PLUGGY_CLIENT_SECRET"].Value, "abc123") {
		t.Error("segredo cifrado no armazenamento")
	}
}

func TestSettingsAPI_PutCSRFAndSecretsNeedKey(t *testing.T) {
	e, s := newSettingsAPI(t, goodKey)
	c := login(t, s)
	if rec := do(s, "PUT", "/api/settings", `{"values":{"DIGEST_HOUR":"8"}}`, map[string]string{"Content-Type": "text/plain"}, c); rec.Code != 403 {
		t.Errorf("sem JSON = %d", rec.Code)
	}
	if rec := do(s, "PUT", "/api/settings", `{"values":{"DIGEST_HOUR":"8"}}`, map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, c); rec.Code != 403 {
		t.Errorf("cross-origin = %d", rec.Code)
	}
	if rec := do(s, "POST", "/api/settings/reset/DIGEST_HOUR", "{}", map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, c); rec.Code != 403 {
		t.Errorf("reset cross-origin = %d", rec.Code)
	}
	if len(e.store.rows) != 0 {
		t.Error("requisições recusadas não gravam")
	}

	_, s2 := newSettingsAPI(t, "")
	c2 := login(t, s2)
	rec := do(s2, "PUT", "/api/settings", `{"values":{"GEMINI_API_KEY":"abc"},"password":"`+testPassword+`"}`, jsonHdr, c2)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "APP_SECRET_KEY") {
		t.Errorf("sem chave mestra: %d %s", rec.Code, rec.Body)
	}
	if rec := do(s2, "GET", "/api/settings", "", nil, c2); !strings.Contains(rec.Body.String(), `"encryption":false`) {
		t.Error("a tela precisa saber que não há criptografia")
	}
}

func TestSettingsAPI_ResetAndRestartPending(t *testing.T) {
	e, s := newSettingsAPI(t, goodKey)
	c := login(t, s)
	e.svc.Set(context.Background(), map[string]string{"DIGEST_HOUR": "3"}) //nolint:errcheck
	rec := do(s, "POST", "/api/settings/reset/DIGEST_HOUR", "{}", jsonHdr, c)
	if rec.Code != 200 || e.svc.Int("DIGEST_HOUR") != 9 {
		t.Errorf("reset: %d %s hora=%d", rec.Code, rec.Body, e.svc.Int("DIGEST_HOUR"))
	}
	if rec := do(s, "POST", "/api/settings/reset/NAO_EXISTE", "{}", jsonHdr, c); rec.Code != 404 {
		t.Errorf("chave desconhecida = %d", rec.Code)
	}

	rec = do(s, "PUT", "/api/settings", `{"values":{"TELEGRAM_CHAT_ID":"999"},"password":"`+testPassword+`"}`, jsonHdr, c)
	if !strings.Contains(rec.Body.String(), `"restartPending":true`) {
		t.Errorf("chat id só vale após reiniciar: %s", rec.Body)
	}
	rec = do(s, "PUT", "/api/settings", `{"values":{"DIGEST_HOUR":"4"}}`, jsonHdr, c)
	if !strings.Contains(rec.Body.String(), `"changed":["DIGEST_HOUR"]`) {
		t.Errorf("%s", rec.Body)
	}
}

func TestSettingsAPI_OwnNamesFindsOldTransfers(t *testing.T) {
	e, s := newSettingsAPI(t, goodKey)
	c := login(t, s)
	mine, other := uuid.New(), uuid.New()
	e.cleaner.cands = []domain.TransferCandidate{
		{PaymentID: mine, Description: "Pix enviado MARIA DA SILVA", Kind: "EXPENSE", Amount: 500},
		{PaymentID: uuid.New(), Description: "Pix recebido Maria da Silva", Kind: "INCOME", Amount: 80},
		{PaymentID: other, Description: "Pix enviado JOAO PEREIRA", Kind: "EXPENSE", Amount: 10},
		{PaymentID: uuid.New(), Description: "Pagamento de boleto MARIA DA SILVA", Kind: "EXPENSE", Amount: 99},
	}

	// Sem mexer nos nomes, nada é perguntado.
	if rec := do(s, "PUT", "/api/settings", `{"values":{"DIGEST_HOUR":"8"}}`, jsonHdr, c); strings.Contains(rec.Body.String(), "ownTransfers") {
		t.Error("só a mudança de OWN_NAMES dispara a pergunta")
	}

	rec := do(s, "PUT", "/api/settings", `{"values":{"OWN_NAMES":"Maria da Silva"}}`, jsonHdr, c)
	body := rec.Body.String()
	for _, want := range []string{`"ownTransfers"`, `"count":2`, `"expense":500`, `"income":80`} {
		if rec.Code != 200 || !strings.Contains(body, want) {
			t.Fatalf("faltou %s em %d %s", want, rec.Code, body)
		}
	}
	if len(e.cleaner.cancelled) != 0 {
		t.Fatal("salvar os nomes não cancela nada: quem decide é a pessoa")
	}

	rec = do(s, "POST", "/api/settings/own-transfers/apply", "{}", jsonHdr, c)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"cancelled":2`) || len(e.cleaner.cancelled) != 2 {
		t.Fatalf("%d %s %v", rec.Code, rec.Body, e.cleaner.cancelled)
	}
	for _, id := range e.cleaner.cancelled {
		if id == other {
			t.Error("transferência para outra pessoa não pode ser cancelada")
		}
	}
	if rec := do(s, "POST", "/api/settings/own-transfers/apply", "{}", map[string]string{"Content-Type": "text/plain"}, c); rec.Code != 403 {
		t.Errorf("sem JSON = %d", rec.Code)
	}
}

func TestSettingsAPI_NoOwnNamesCancelsNothing(t *testing.T) {
	e, s := newSettingsAPI(t, goodKey)
	c := login(t, s)
	e.cleaner.cands = []domain.TransferCandidate{{PaymentID: uuid.New(), Description: "Pix enviado QUALQUER", Kind: "EXPENSE", Amount: 1}}
	rec := do(s, "POST", "/api/settings/own-transfers/apply", "{}", jsonHdr, c)
	if rec.Code != 200 || len(e.cleaner.cancelled) != 0 {
		t.Errorf("sem nomes configurados nada casa: %d %v", rec.Code, e.cleaner.cancelled)
	}
}

func TestSettingsAPI_TestConnections(t *testing.T) {
	e, s := newSettingsAPI(t, goodKey)
	c := login(t, s)
	post := func(target string) string {
		return do(s, "POST", "/api/settings/test/"+target, "{}", jsonHdr, c).Body.String()
	}

	if got := post("pluggy"); !strings.Contains(got, `"ok":false`) {
		t.Errorf("sem credenciais: %s", got)
	}
	e.svc.Set(context.Background(), map[string]string{"PLUGGY_CLIENT_ID": "id", "PLUGGY_CLIENT_SECRET": "bom", "PLUGGY_ITEM_IDS": "592c8fdb-a1b3-495c-b2d6-ec9027cf386d"}) //nolint:errcheck
	if got := post("pluggy"); !strings.Contains(got, `"ok":true`) || !strings.Contains(got, "1 conexão") {
		t.Errorf("credenciais boas: %s", got)
	}
	e.svc.Set(context.Background(), map[string]string{"PLUGGY_ITEM_IDS": "592c8fdb-a1b3-495c-b2d6-ec9027cf386d,00000000-0000-0000-0000-000000000000"}) //nolint:errcheck
	if got := post("pluggy"); !strings.Contains(got, `"ok":false`) || !strings.Contains(got, "1 Item ID") {
		t.Errorf("item inválido: %s", got)
	}
	e.svc.Set(context.Background(), map[string]string{"PLUGGY_CLIENT_SECRET": "ruim"}) //nolint:errcheck
	got := post("pluggy")
	if !strings.Contains(got, `"ok":false`) || strings.Contains(got, "vazado") || strings.Contains(got, "ruim") {
		t.Errorf("erro do serviço não pode ser repetido: %s", got)
	}

	e.svc.Set(context.Background(), map[string]string{"GEMINI_API_KEY": "ruim"}) //nolint:errcheck
	if got := post("gemini"); !strings.Contains(got, `"ok":false`) || strings.Contains(got, "vazada") {
		t.Errorf("gemini ruim: %s", got)
	}
	e.svc.Set(context.Background(), map[string]string{"GEMINI_API_KEY": "boa", "TELEGRAM_BOT_TOKEN": "tok"}) //nolint:errcheck
	if got := post("gemini"); !strings.Contains(got, `"ok":true`) {
		t.Errorf("gemini bom: %s", got)
	}
	if got := post("telegram"); !strings.Contains(got, "@meu_bot") {
		t.Errorf("telegram: %s", got)
	}
	e.svc.Set(context.Background(), map[string]string{"TELEGRAM_BOT_TOKEN": "ruim"}) //nolint:errcheck
	if got := post("telegram"); !strings.Contains(got, `"ok":false`) || strings.Contains(got, "vazado") {
		t.Errorf("telegram ruim: %s", got)
	}
	if rec := do(s, "POST", "/api/settings/test/outro", "{}", jsonHdr, c); rec.Code != 404 {
		t.Errorf("alvo desconhecido = %d", rec.Code)
	}
}

func TestSettingsAPI_Restart(t *testing.T) {
	e, s := newSettingsAPI(t, goodKey)
	c := login(t, s)
	if rec := do(s, "POST", "/api/restart", "{}", map[string]string{"Content-Type": "text/plain"}, c); rec.Code != 403 || e.restart.Load() != 0 {
		t.Errorf("sem JSON = %d", rec.Code)
	}
	if rec := do(s, "POST", "/api/restart", "{}", jsonHdr, c); rec.Code != 200 {
		t.Fatalf("%d", rec.Code)
	}
	deadline := time.Now().Add(3 * time.Second)
	for e.restart.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if e.restart.Load() != 1 {
		t.Error("deveria pedir o reinício")
	}
}

// O Coach consulta a configuração a cada uso: ligar o plano pago na página vale sem reiniciar.
func TestCoachService_PaidFollowsFunc(t *testing.T) {
	paid := false
	c := newCoachService(&fakeCoach{}, false)
	c.paidFn = func() bool { return paid }
	if c.enabled() {
		t.Fatal("bloqueado sem plano pago")
	}
	paid = true
	if !c.enabled() {
		t.Error("deveria liberar quando a configuração mudar")
	}
}

const withPW = `,"password":"` + testPassword + `"}`

// Mexer no que é sensível exige a senha do dashboard de novo; uma sessão roubada não basta.
func TestSettingsAPI_SensitiveNeedsPassword(t *testing.T) {
	e, s := newSettingsAPI(t, goodKey)
	c := login(t, s)

	for _, key := range []string{"PLUGGY_CLIENT_SECRET", "PLUGGY_ITEM_IDS", "TELEGRAM_CHAT_ID"} { // as tentativas erradas têm limite
		val := "abc123"
		if key == "PLUGGY_ITEM_IDS" {
			val = "592c8fdb-a1b3-495c-b2d6-ec9027cf386d"
		}
		if key == "TELEGRAM_CHAT_ID" {
			val = "123"
		}
		bodies := []string{`{"values":{"` + key + `":"` + val + `"}}`}
		if key == "PLUGGY_CLIENT_SECRET" {
			bodies = append(bodies, `{"values":{"`+key+`":"`+val+`"},"password":"errada"}`)
		}
		for _, body := range bodies {
			rec := do(s, "PUT", "/api/settings", body, jsonHdr, c)
			if rec.Code != 403 || !strings.Contains(rec.Body.String(), "senha incorreta") {
				t.Errorf("%s: %d %s", key, rec.Code, rec.Body)
			}
		}
	}
	if len(e.store.rows) != 0 {
		t.Fatalf("sem a senha nada é gravado: %v", e.store.rows)
	}

	// Configuração comum não pede senha, mesmo no mesmo lote só se nenhuma chave for sensível.
	if rec := do(s, "PUT", "/api/settings", `{"values":{"DIGEST_HOUR":"8"}}`, jsonHdr, c); rec.Code != 200 {
		t.Errorf("comum sem senha = %d", rec.Code)
	}
	if rec := do(s, "PUT", "/api/settings", `{"values":{"DIGEST_HOUR":"7","PLUGGY_CLIENT_SECRET":"x1"}}`, jsonHdr, c); rec.Code != 403 || e.svc.Int("DIGEST_HOUR") != 8 {
		t.Errorf("uma chave sensível no lote exige a senha para o lote todo: %d", rec.Code)
	}

	rec := do(s, "PUT", "/api/settings", `{"values":{"PLUGGY_CLIENT_SECRET":"abc123"}`+withPW, jsonHdr, c)
	if rec.Code != 200 || e.svc.Get("PLUGGY_CLIENT_SECRET") != "abc123" {
		t.Fatalf("com a senha certa: %d %s", rec.Code, rec.Body)
	}
}

func TestSettingsAPI_ResetSensitiveNeedsPassword(t *testing.T) {
	e, s := newSettingsAPI(t, goodKey)
	c := login(t, s)
	e.svc.Set(context.Background(), map[string]string{"PLUGGY_CLIENT_SECRET": "abc123"}) //nolint:errcheck

	if rec := do(s, "POST", "/api/settings/reset/PLUGGY_CLIENT_SECRET", `{"password":"errada"}`, jsonHdr, c); rec.Code != 403 || e.svc.Get("PLUGGY_CLIENT_SECRET") == "" {
		t.Errorf("restaurar segredo exige a senha: %d", rec.Code)
	}
	if rec := do(s, "POST", "/api/settings/reset/PLUGGY_CLIENT_SECRET", `{"password":"`+testPassword+`"}`, jsonHdr, c); rec.Code != 200 || e.svc.Get("PLUGGY_CLIENT_SECRET") != "" {
		t.Errorf("com a senha: %d", rec.Code)
	}
	e.svc.Set(context.Background(), map[string]string{"DIGEST_HOUR": "3"}) //nolint:errcheck
	if rec := do(s, "POST", "/api/settings/reset/DIGEST_HOUR", "{}", jsonHdr, c); rec.Code != 200 {
		t.Errorf("restaurar o que é comum não pede senha: %d", rec.Code)
	}
}

func TestSettingsAPI_AuditAndNotice(t *testing.T) {
	e, s := newSettingsAPI(t, goodKey)
	c := login(t, s)
	if len(e.notices) != 1 || !strings.Contains(e.notices[0], "Login no dashboard") {
		t.Fatalf("o login avisa no chat: %v", e.notices)
	}
	e.notices = nil

	do(s, "PUT", "/api/settings", `{"values":{"DIGEST_HOUR":"8"}}`, jsonHdr, c)
	if len(e.notices) != 0 {
		t.Errorf("mudança comum não avisa no chat: %v", e.notices)
	}
	do(s, "PUT", "/api/settings", `{"values":{"TELEGRAM_BOT_TOKEN":"tok-super-secreto"}`+withPW, jsonHdr, c)
	if len(e.notices) != 1 || !strings.Contains(e.notices[0], "Token do bot") || strings.Contains(e.notices[0], "tok-super-secreto") {
		t.Fatalf("aviso sobre o que mudou, sem o valor: %v", e.notices)
	}

	rec := do(s, "GET", "/api/settings/audit", "", nil, c)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `"key":"TELEGRAM_BOT_TOKEN"`) || !strings.Contains(body, `"sensitive":true`) || !strings.Contains(body, `"label":"Token do bot"`) {
		t.Fatalf("histórico: %d %s", rec.Code, body)
	}
	if strings.Contains(body, "tok-super-secreto") {
		t.Fatal("o histórico nunca guarda valores")
	}
}

func TestSettingsAPI_WrongPasswordNoticeIsThrottledAndLimited(t *testing.T) {
	e, s := newSettingsAPI(t, goodKey)
	c := login(t, s)
	e.notices = nil
	bad := `{"values":{"PLUGGY_CLIENT_SECRET":"x"},"password":"errada"}`
	var last int
	for i := 0; i < 10; i++ {
		last = do(s, "PUT", "/api/settings", bad, jsonHdr, c).Code
	}
	if len(e.notices) != 1 || !strings.Contains(e.notices[0], "errou a senha") {
		t.Errorf("um aviso só por janela, got %d: %v", len(e.notices), e.notices)
	}
	if last != 429 {
		t.Errorf("tentativas de senha têm limite, último = %d", last)
	}
}
