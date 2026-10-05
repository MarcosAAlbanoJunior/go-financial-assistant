package setup

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
)

// memStore imita o banco, inclusive a checagem otimista da conclusão.
type memStore struct {
	mu        sync.Mutex
	owner     Owner
	draft     Draft
	rows      map[string]settings.Row
	completes int
	cleared   bool
	failLoad  error
}

func (m *memStore) LoadOwner(context.Context) (Owner, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.owner, m.failLoad
}
func (m *memStore) LoadDraft(context.Context) (Draft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.draft, nil
}
func (m *memStore) SaveDraft(_ context.Context, d Draft) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.draft = d
	return nil
}
func (m *memStore) Complete(_ context.Context, c Completion) (*time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	same := (m.owner.CompletedAt == nil) == (c.Expected.CompletedAt == nil) &&
		(m.owner.CompletedAt == nil || m.owner.CompletedAt.Equal(*c.Expected.CompletedAt)) && m.owner.ReopenDone == c.Expected.ReopenDone
	if !same {
		return nil, nil
	}
	time.Sleep(5 * time.Millisecond) // alarga a janela da corrida nos testes de concorrência
	now := time.Now()
	m.owner.CompletedAt = &now
	if c.PasswordHash != "" {
		m.owner.PasswordHash = c.PasswordHash
	}
	m.owner.ReopenDone = m.owner.ReopenDone || c.Reopen
	for _, r := range c.Rows {
		m.rows[r.Key] = r
	}
	m.draft = Draft{}
	m.completes++
	return &now, nil
}
func (m *memStore) ClearReopen(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.owner.ReopenDone, m.cleared = false, true
	return nil
}

const (
	testToken = "um-token-de-setup-com-32-caracteres!"
	masterKey = "uma-chave-mestra-de-teste"
)

func newTestService(t *testing.T, store *memStore, env Env) *Service {
	t.Helper()
	if store.rows == nil {
		store.rows = map[string]settings.Row{}
	}
	cipher, err := settings.NewCipher(masterKey)
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(store, cipher, env, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := s.Load(context.Background()); err != nil && store.failLoad == nil {
		t.Fatal(err)
	}
	return s
}

func ready(v bool) func() bool { return func() bool { return v } }

// readyToConfirm leva o setup até o ponto de digitar o código: senha, bot e "sim, sou eu".
func readyToConfirm(t *testing.T, s *Service) {
	t.Helper()
	ctx := context.Background()
	if err := s.SetPassword(ctx, "senha-nova-do-dono"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBot(ctx, "123:TOKEN", "meu_bot", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetCandidate(ctx, "123:TOKEN", Candidate{ID: 42, Name: "Dono"}, 11); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AcceptCandidate(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestOpen_Rules(t *testing.T) {
	cases := []struct {
		name    string
		env     Env
		owner   Owner
		channel bool
		open    bool
	}{
		{"instalação nova", Env{}, Owner{}, false, true},
		{"instalação atual com .env completo", Env{EnvPassword: "senha-do-env-12"}, Owner{}, true, false},
		{"senha no .env, sem canal", Env{EnvPassword: "senha-do-env-12"}, Owner{}, false, true},
		{"canal no .env, sem senha (API desligada)", Env{}, Owner{}, true, true},
		{"senha salva sem concluir não conta", Env{}, Owner{PasswordHash: "$argon2id$x"}, true, true},
		{"reaberto", Env{EnvPassword: "senha-do-env-12", Reopen: true}, Owner{}, true, true},
		{"reaberto e já refeito", Env{EnvPassword: "senha-do-env-12", Reopen: true}, Owner{ReopenDone: true}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.env.ChannelReady = ready(tc.channel)
			s := newTestService(t, &memStore{owner: tc.owner}, tc.env)
			if s.Open() != tc.open {
				t.Fatalf("Open = %v, queria %v", s.Open(), tc.open)
			}
		})
	}
}

func TestCheckToken(t *testing.T) {
	cases := []struct {
		token, try string
		noCipher   bool
		want       error
	}{
		{"", "x", false, ErrNoToken},
		{"curto-demais", "curto-demais", false, ErrTokenTooShort},
		{testToken, testToken, true, ErrNoEncryption},
		{testToken, testToken + "x", false, ErrWrongToken},
		{testToken, testToken, false, nil},
	}
	for _, tc := range cases {
		s := newTestService(t, &memStore{}, Env{Token: tc.token})
		if tc.noCipher {
			s.cipher = nil
		}
		if err := s.CheckToken(tc.try); !errors.Is(err, tc.want) {
			t.Errorf("token %q, tentativa %q: %v, queria %v", tc.token, tc.try, err, tc.want)
		}
	}
}

func TestTokenProblem_MissingMigration(t *testing.T) {
	s := newTestService(t, &memStore{failLoad: errors.New(`relation "dashboard_owner" does not exist`)}, Env{Token: testToken})
	if err := s.TokenProblem(); err == nil || !strings.Contains(err.Error(), "migration 019") {
		t.Fatalf("sem a migration: %v", err)
	}
}

func TestPassword_OnlyValidAfterCompletion(t *testing.T) {
	store := &memStore{}
	s := newTestService(t, store, Env{Token: testToken})
	ctx := context.Background()
	if err := s.SetPassword(ctx, "curta"); !errors.Is(err, ErrPasswordShort) {
		t.Fatalf("senha curta: %v", err)
	}
	readyToConfirm(t, s)
	if strings.Contains(store.draft.PasswordHash, "senha-nova") || !strings.HasPrefix(store.draft.PasswordHash, "$argon2id$") {
		t.Fatalf("o rascunho guarda o hash, nunca a senha: %q", store.draft.PasswordHash)
	}
	if s.CheckPassword("senha-nova-do-dono") {
		t.Fatal("senha do rascunho não pode entrar antes de concluir")
	}
	if _, err := s.CompleteTelegram(ctx); err != nil {
		t.Fatal(err)
	}
	if s.Open() || !s.CheckPassword("senha-nova-do-dono") || s.CheckPassword("outra-senha-qualquer") {
		t.Fatal("depois de concluir, vale a senha nova (e só ela)")
	}
}

func TestCheckPassword_DBWinsOverEnv(t *testing.T) {
	store := &memStore{}
	s := newTestService(t, store, Env{Token: testToken, EnvPassword: "senha-do-env-12", Reopen: true, ChannelReady: ready(true)})
	if s.CheckPassword("senha-do-env-12") {
		t.Fatal("reaberto, nenhuma senha entra")
	}
	readyToConfirm(t, s)
	if _, err := s.CompleteTelegram(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.CheckPassword("senha-do-env-12") || !s.CheckPassword("senha-nova-do-dono") {
		t.Fatal("a senha do setup vale mais que a do ambiente (salvo > ambiente)")
	}
}

func TestCheckPassword_EnvWhenNoSetupPassword(t *testing.T) {
	s := newTestService(t, &memStore{}, Env{EnvPassword: "senha-do-env-12", ChannelReady: ready(true)})
	if !s.CheckPassword("senha-do-env-12") || s.CheckPassword("senha-do-env-1") {
		t.Fatal("sem setup, vale a DASHBOARD_PASSWORD")
	}
}

func TestTelegramDraft_Flow(t *testing.T) {
	store := &memStore{}
	s := newTestService(t, store, Env{Token: testToken})
	ctx := context.Background()

	if _, err := s.SetCandidate(ctx, "123:TOKEN", Candidate{ID: 1}, 5); !errors.Is(err, ErrNoBot) {
		t.Fatalf("sem bot: %v", err)
	}
	if err := s.SetBot(ctx, "123:TOKEN", "meu_bot", 10); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(store.draft.TelegramToken, "TOKEN") {
		t.Fatal("o token do bot fica cifrado no rascunho")
	}
	if _, err := s.AcceptCandidate(ctx); !errors.Is(err, ErrNoCandidate) {
		t.Fatalf("aceitar sem ninguém: %v", err)
	}
	// Outra pessoa manda /start primeiro; uma segunda mensagem não troca quem está na tela.
	if _, err := s.SetCandidate(ctx, "123:TOKEN", Candidate{ID: 7, Name: "Outra Pessoa"}, 11); err != nil {
		t.Fatal(err)
	}
	if d, _ := s.SetCandidate(ctx, "123:TOKEN", Candidate{ID: 8}, 12); d.Candidate.ID != 7 || d.TelegramOffset != 12 {
		t.Fatalf("o primeiro fica até a resposta: %+v", d)
	}
	if err := s.RejectCandidate(ctx); err != nil {
		t.Fatal(err)
	}
	st, _ := s.Status(ctx)
	if st.Draft.Candidate != nil || st.Draft.TelegramOffset != 12 || st.Draft.TelegramToken != "123:TOKEN" {
		t.Fatalf("\"não sou eu\" descarta e segue esperando depois dela: %+v", st.Draft)
	}
	if _, err := s.SetCandidate(ctx, "123:TOKEN", Candidate{ID: 42, Name: "Dono"}, 13); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AcceptCandidate(ctx); err != nil {
		t.Fatal(err)
	}
	// Resposta atrasada do bot anterior não vale para o bot novo.
	if err := s.SetBot(ctx, "999:OUTRO", "outro_bot", 50); err != nil {
		t.Fatal(err)
	}
	if d, _ := s.SetCandidate(ctx, "123:TOKEN", Candidate{ID: 9}, 99); d.Candidate != nil || d.TelegramOffset != 50 {
		t.Fatalf("candidato de outro bot: %+v", d)
	}
	s.SetBot(ctx, "123:TOKEN", "meu_bot", 13)                             //nolint:errcheck
	s.SetCandidate(ctx, "123:TOKEN", Candidate{ID: 42, Name: "Dono"}, 14) //nolint:errcheck
	s.AcceptCandidate(ctx)                                                //nolint:errcheck
	// Trocar de bot descarta o ID recebido.
	if err := s.SetBot(ctx, "999:OUTRO", "outro_bot", 50); err != nil {
		t.Fatal(err)
	}
	st, _ = s.Status(ctx)
	if st.Draft.Candidate != nil || st.Draft.CandidateAccepted || st.Draft.TelegramOffset != 50 {
		t.Fatalf("trocar o bot recomeça a espera: %+v", st.Draft)
	}
}

// Ao concluir, o canal vem do rascunho confirmado e é gravado com o token cifrado; o rascunho some.
func TestCompleteTelegram_FromDraft(t *testing.T) {
	store := &memStore{}
	s := newTestService(t, store, Env{Token: testToken})
	readyToConfirm(t, s)
	act, err := s.CompleteTelegram(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if act.Token != "123:TOKEN" || act.ChatID != 42 || act.Offset != 11 || act.Keep {
		t.Fatalf("ativação: %+v", act)
	}
	if store.rows["CHANNEL"].Value != "telegram" || store.rows["TELEGRAM_CHAT_ID"].Value != "42" || !store.rows["TELEGRAM_BOT_TOKEN"].Secret {
		t.Fatalf("canal gravado: %+v", store.rows)
	}
	plain, err := s.cipher.Decrypt("TELEGRAM_BOT_TOKEN", store.rows["TELEGRAM_BOT_TOKEN"].Value)
	if err != nil || plain != "123:TOKEN" {
		t.Fatalf("token gravado cifrado com a chave do campo: %q %v", plain, err)
	}
	if store.draft.TelegramToken != "" || store.draft.PasswordHash != "" {
		t.Fatal("o rascunho é apagado ao concluir")
	}
	if _, err := s.CompleteTelegram(context.Background()); !errors.Is(err, ErrAlreadyComplete) {
		t.Fatalf("concluir de novo: %v", err)
	}
	if err := s.SetPassword(context.Background(), "mais-uma-senha-longa"); !errors.Is(err, ErrClosed) {
		t.Fatalf("depois de concluído, os passos fecham: %v", err)
	}
}

func TestCompleteTelegram_NeedsEveryStep(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t, &memStore{}, Env{Token: testToken})
	if _, err := s.CompleteTelegram(ctx); !errors.Is(err, ErrNoPassword) {
		t.Fatalf("sem senha: %v", err)
	}
	s.SetPassword(ctx, "senha-nova-do-dono") //nolint:errcheck
	if _, err := s.CompleteTelegram(ctx); !errors.Is(err, ErrNoBot) {
		t.Fatalf("sem bot: %v", err)
	}
	s.SetBot(ctx, "123:TOKEN", "meu_bot", 10) //nolint:errcheck
	if _, err := s.CompleteTelegram(ctx); !errors.Is(err, ErrNoCandidate) {
		t.Fatalf("sem /start: %v", err)
	}
	s.SetCandidate(ctx, "123:TOKEN", Candidate{ID: 42}, 11) //nolint:errcheck
	if _, err := s.CompleteTelegram(ctx); !errors.Is(err, ErrNotAccepted) {
		t.Fatalf("sem \"sou eu\": %v", err)
	}
}

// Duas conclusões ao mesmo tempo: uma grava, a outra recebe "já concluído" (um bot só).
func TestComplete_Concurrent(t *testing.T) {
	store := &memStore{}
	s := newTestService(t, store, Env{Token: testToken})
	readyToConfirm(t, s)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = s.CompleteTelegram(context.Background())
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else if !errors.Is(err, ErrAlreadyComplete) {
			t.Fatalf("erro inesperado: %v", err)
		}
	}
	if ok != 1 || store.completes != 1 {
		t.Fatalf("concluíram %d (gravações %d), queria 1", ok, store.completes)
	}
}

// Mesmo com dois processos (o mutex não protege), a checagem otimista do banco só deixa um gravar.
func TestComplete_StaleOwnerDoesNotWrite(t *testing.T) {
	store := &memStore{}
	a := newTestService(t, store, Env{Token: testToken})
	b := newTestService(t, store, Env{Token: testToken})
	readyToConfirm(t, a)
	if _, err := a.CompleteTelegram(context.Background()); err != nil {
		t.Fatal(err)
	}
	readyToConfirm(t, b)
	if _, err := b.CompleteTelegram(context.Background()); !errors.Is(err, ErrAlreadyComplete) || store.completes != 1 {
		t.Fatalf("segundo processo: %v (gravações %d)", err, store.completes)
	}
}

func TestCompleteKeep(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t, &memStore{}, Env{Token: testToken, EnvPassword: "senha-do-env-12"})
	if err := s.CompleteKeep(ctx); !errors.Is(err, ErrNoChannel) {
		t.Fatalf("sem canal para manter: %v", err)
	}
	store := &memStore{}
	s = newTestService(t, store, Env{Token: testToken, ChannelReady: ready(true)})
	if err := s.SetBot(ctx, "123:TOKEN", "meu_bot", 0); !errors.Is(err, ErrChannelInEnv) {
		t.Fatalf("com canal no .env, o setup não liga outro: %v", err)
	}
	if err := s.CompleteKeep(ctx); !errors.Is(err, ErrNoPassword) {
		t.Fatalf("sem senha: %v", err)
	}
	s.SetPassword(ctx, "senha-nova-do-dono") //nolint:errcheck
	if err := s.CompleteKeep(ctx); err != nil || s.Open() || len(store.rows) != 0 {
		t.Fatalf("manter o canal: %v open=%v rows=%v", err, s.Open(), store.rows)
	}
}

func TestReopen(t *testing.T) {
	ctx := context.Background()
	done := time.Now()
	store := &memStore{owner: Owner{PasswordHash: "$argon2id$antiga", CompletedAt: &done}}
	env := Env{Token: testToken, Reopen: true, ChannelReady: ready(true)}
	s := newTestService(t, store, env)
	if !s.Open() || !s.Reopen() {
		t.Fatal("SETUP_REOPEN reabre o setup")
	}
	st, _ := s.Status(ctx)
	if !st.PasswordCurrent || !st.ChannelInEnv {
		t.Fatalf("reaberto, pode manter a senha e o canal: %+v", st)
	}
	// Mantém a senha de antes e troca o canal.
	if err := s.SetBot(ctx, "123:TOKEN", "meu_bot", 0); err != nil {
		t.Fatal(err)
	}
	s.SetCandidate(ctx, "123:TOKEN", Candidate{ID: 42}, 1) //nolint:errcheck
	s.AcceptCandidate(ctx)                                 //nolint:errcheck
	if _, err := s.CompleteTelegram(ctx); err != nil {
		t.Fatal(err)
	}
	if s.Open() || !store.owner.ReopenDone || store.owner.PasswordHash != "$argon2id$antiga" {
		t.Fatalf("refeito: open=%v owner=%+v", s.Open(), store.owner)
	}
	// Reinício com SETUP_REOPEN ainda ligado: não reabre em loop.
	if s = newTestService(t, store, env); s.Open() {
		t.Fatal("SETUP_REOPEN esquecido não reabre de novo")
	}
	// Reinício sem SETUP_REOPEN libera uma próxima reabertura.
	env.Reopen = false
	newTestService(t, store, env)
	if !store.cleared || store.owner.ReopenDone {
		t.Fatal("sem SETUP_REOPEN, a marca de refeito é limpa")
	}
	env.Reopen = true
	if s = newTestService(t, store, env); !s.Open() {
		t.Fatal("ligar SETUP_REOPEN de novo reabre")
	}
}
