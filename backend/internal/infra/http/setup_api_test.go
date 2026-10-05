package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/setup"
)

const (
	setupTok   = "um-token-de-setup-com-32-caracteres!"
	botToken   = "123456:ABC-bot-token"
	ownerPass  = "senha-do-dono-123"
	ownerID    = int64(4242)
	strangerID = int64(777)
)

// memSetupStore imita o banco do setup, com a checagem otimista da conclusão.
type memSetupStore struct {
	mu    sync.Mutex
	owner setup.Owner
	draft setup.Draft
	rows  []settings.Row
}

func (m *memSetupStore) LoadOwner(context.Context) (setup.Owner, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.owner, nil
}
func (m *memSetupStore) LoadDraft(context.Context) (setup.Draft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.draft, nil
}
func (m *memSetupStore) SaveDraft(_ context.Context, d setup.Draft) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.draft = d
	return nil
}
func (m *memSetupStore) Complete(_ context.Context, c setup.Completion) (*time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if (m.owner.CompletedAt == nil) != (c.Expected.CompletedAt == nil) || m.owner.ReopenDone != c.Expected.ReopenDone {
		return nil, nil
	}
	now := time.Now()
	m.owner.CompletedAt = &now
	if c.PasswordHash != "" {
		m.owner.PasswordHash = c.PasswordHash
	}
	m.owner.ReopenDone = m.owner.ReopenDone || c.Reopen
	m.rows = append(m.rows, c.Rows...)
	m.draft = setup.Draft{}
	return &now, nil
}
func (m *memSetupStore) ClearReopen(context.Context) error { return nil }

type tgUpdate struct {
	id     int64
	sender setup.Candidate
}

// fakeTelegram é o Telegram do setup: bots conhecidos, mensagens chegando e o que foi enviado.
type fakeTelegram struct {
	mu      sync.Mutex
	latest  int64 // offset seguinte à última mensagem antiga (antes de colar o token)
	updates []tgUpdate
	sent    map[int64][]string
	down    bool
}

func (f *fakeTelegram) Bot(_ context.Context, token string) (string, error) {
	switch {
	case f.down:
		return "", setup.ErrTelegramDown
	case token != botToken:
		return "", setup.ErrBotTokenInvalid
	}
	return "meu_bot", nil
}
func (f *fakeTelegram) LatestOffset(context.Context, string) (int64, error) { return f.latest, nil }
func (f *fakeTelegram) FirstPrivate(_ context.Context, _ string, offset int64) (*setup.Candidate, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.updates {
		if u.id >= offset {
			c := u.sender
			return &c, u.id + 1, nil
		}
	}
	return nil, offset, nil
}
func (f *fakeTelegram) Send(_ context.Context, _ string, chatID int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent[chatID] = append(f.sent[chatID], text)
	return nil
}
func (f *fakeTelegram) arrive(id int64, c setup.Candidate) {
	f.mu.Lock()
	f.updates = append(f.updates, tgUpdate{id, c})
	f.mu.Unlock()
}
func (f *fakeTelegram) lastCode(t *testing.T, chatID int64) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	msgs := f.sent[chatID]
	if len(msgs) == 0 {
		t.Fatalf("nenhum código enviado ao chat %d", chatID)
	}
	return codeRe.FindString(msgs[len(msgs)-1])
}

type setupEnv struct {
	s           *Server
	svc         *setup.Service
	store       *memSetupStore
	tg          *fakeTelegram
	audit       *memAudit
	box         *chatBox // o canal do app (segundo fator) depois de ligado
	channelOn   atomic.Bool
	mu          sync.Mutex
	activations []setup.Activation
	activateErr error
}

func newSetupEnv(t *testing.T, env setup.Env, owner setup.Owner) *setupEnv {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cipher, err := settings.NewCipher(goodKey)
	if err != nil {
		t.Fatal(err)
	}
	e := &setupEnv{store: &memSetupStore{owner: owner}, tg: &fakeTelegram{latest: 100, sent: map[int64][]string{}}, audit: &memAudit{}, box: &chatBox{}}
	if env.ChannelReady == nil {
		env.ChannelReady = e.channelOn.Load
	}
	e.svc = setup.NewService(e.store, cipher, env, logger)
	if err := e.svc.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.s = NewServer(0, logger)
	e.s.SetSetup(&SetupDeps{
		Service: e.svc, Telegram: e.tg, Audit: e.audit,
		Activate: func(_ context.Context, a setup.Activation) error {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.activations = append(e.activations, a)
			e.channelOn.Store(true)
			return e.activateErr
		},
	})
	e.s.SetSecondFactor(&SecondFactor{Enabled: true, Send: e.box.send, Configured: e.channelOn.Load})
	if err := e.s.MountAPI(e.svc.CheckPassword, &fakeReader{}); err != nil {
		t.Fatal(err)
	}
	return e
}

func newFreshSetup(t *testing.T) *setupEnv {
	t.Helper()
	return newSetupEnv(t, setup.Env{Token: setupTok}, setup.Owner{})
}

func (e *setupEnv) status(t *testing.T, cookies ...*http.Cookie) map[string]any {
	t.Helper()
	rec := do(e.s, "GET", "/api/setup/status", "", nil, cookies...)
	if rec.Code != 200 {
		t.Fatalf("status = %d %s", rec.Code, rec.Body)
	}
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out) //nolint:errcheck
	return out
}

func (e *setupEnv) post(path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	return do(e.s, "POST", path, body, jsonHdr, cookies...)
}

// openSession cola o token e devolve o cookie da sessão de setup.
func (e *setupEnv) openSession(t *testing.T) *http.Cookie {
	t.Helper()
	rec := do(e.s, "POST", "/api/setup/token", `{"token":"`+setupTok+`"}`, jsonHdr)
	c := cookieNamed(rec, setupCookie)
	if rec.Code != 200 || c == nil {
		t.Fatalf("token certo = %d %s", rec.Code, rec.Body)
	}
	return c
}

// untilCode leva o setup até o código enviado ao chat do dono.
func (e *setupEnv) untilCode(t *testing.T, c *http.Cookie) {
	t.Helper()
	steps := []struct{ path, body string }{
		{"/api/setup/password", `{"password":"` + ownerPass + `"}`},
		{"/api/setup/telegram/bot", `{"token":"` + botToken + `"}`},
	}
	for _, st := range steps {
		if rec := do(e.s, "POST", st.path, st.body, jsonHdr, c); rec.Code != 200 {
			t.Fatalf("%s = %d %s", st.path, rec.Code, rec.Body)
		}
	}
	e.tg.arrive(100, setup.Candidate{ID: ownerID, Name: "Dono da Conta", Username: "dono"})
	if rec := do(e.s, "POST", "/api/setup/telegram/poll", "{}", jsonHdr, c); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Dono da Conta") {
		t.Fatalf("poll = %d %s", rec.Code, rec.Body)
	}
	if rec := do(e.s, "POST", "/api/setup/telegram/accept", "{}", jsonHdr, c); rec.Code != 200 {
		t.Fatalf("accept = %d %s", rec.Code, rec.Body)
	}
}

func TestSetup_GateBlocksTheRestOfTheAPI(t *testing.T) {
	e := newFreshSetup(t)
	for _, p := range [][2]string{{"GET", "/api/me"}, {"POST", "/api/login"}, {"GET", "/api/summary?month=2026-09"}, {"GET", "/api/settings"}} {
		rec := do(e.s, p[0], p[1], `{"password":"x"}`, jsonHdr)
		if rec.Code != 409 || !strings.Contains(rec.Body.String(), `"setup":"required"`) {
			t.Errorf("%s %s com setup aberto = %d %s", p[0], p[1], rec.Code, rec.Body)
		}
	}
	if rec := do(e.s, "GET", "/health", "", nil); rec.Code != 200 {
		t.Errorf("/health = %d", rec.Code)
	}
	if st := e.status(t); st["open"] != true || st["session"] != false || st["password"] != nil {
		t.Errorf("status sem sessão não mostra os passos: %v", st)
	}
}

func TestSetup_WithoutTokenOnlyExplains(t *testing.T) {
	e := newSetupEnv(t, setup.Env{}, setup.Owner{})
	if st := e.status(t); st["problemKind"] != "no-token" || !strings.Contains(st["problem"].(string), "SETUP_TOKEN") {
		t.Fatalf("sem SETUP_TOKEN: %v", st)
	}
	if rec := do(e.s, "POST", "/api/setup/token", `{"token":""}`, jsonHdr); rec.Code != 409 || cookieNamed(rec, setupCookie) != nil {
		t.Fatalf("token sem SETUP_TOKEN = %d %s", rec.Code, rec.Body)
	}
	short := newSetupEnv(t, setup.Env{Token: "curto"}, setup.Owner{})
	if st := short.status(t); st["problemKind"] != "short-token" {
		t.Fatalf("token curto: %v", st)
	}
}

func TestSetup_WrongTokenSixTimesIs429(t *testing.T) {
	e := newFreshSetup(t)
	for i := 1; i <= 6; i++ {
		rec := do(e.s, "POST", "/api/setup/token", `{"token":"errado-errado-errado-errado"}`, jsonHdr)
		want := 401
		if i == 6 {
			want = 429
		}
		if rec.Code != want || cookieNamed(rec, setupCookie) != nil {
			t.Fatalf("tentativa %d = %d, queria %d", i, rec.Code, want)
		}
	}
}

func TestSetup_SessionCookieAndProtections(t *testing.T) {
	e := newFreshSetup(t)
	c := e.openSession(t)
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/api/setup" || c.MaxAge != 3600 {
		t.Fatalf("cookie de setup: %+v", c)
	}
	if rec := do(e.s, "POST", "/api/setup/password", `{"password":"`+ownerPass+`"}`, jsonHdr); rec.Code != 401 {
		t.Fatalf("passo sem sessão = %d", rec.Code)
	}
	cross := map[string]string{"Content-Type": "application/json", "Origin": "https://outro.site"}
	if rec := do(e.s, "POST", "/api/setup/password", `{"password":"`+ownerPass+`"}`, cross, c); rec.Code != 403 {
		t.Fatalf("outra origem = %d", rec.Code)
	}
	if rec := do(e.s, "POST", "/api/setup/password", `{"password":"`+ownerPass+`"}`, nil, c); rec.Code != 403 {
		t.Fatalf("sem JSON = %d", rec.Code)
	}
	// A sessão de setup não abre nada além do setup.
	if rec := do(e.s, "GET", "/api/me", "", nil, c); rec.Code != 409 {
		t.Fatalf("/api/me com sessão de setup = %d", rec.Code)
	}
	rec := do(e.s, "POST", "/api/setup/password", `{"password":"curta"}`, jsonHdr, c)
	if rec.Code != 400 || cookieNamed(rec, setupCookie) == nil {
		t.Fatalf("senha curta = %d (e a sessão é renovada a cada passo)", rec.Code)
	}
}

// Cada linha da tabela "Sair no meio" da spec.
func TestSetup_LeavingMidway(t *testing.T) {
	t.Run("antes de colar o token: nada salvo", func(t *testing.T) {
		e := newFreshSetup(t)
		if st := e.status(t); st["session"] != false || e.store.draft.PasswordHash != "" {
			t.Fatalf("nada deveria existir: %v", st)
		}
	})

	t.Run("token aceito, sem senha: pede o token de novo", func(t *testing.T) {
		e := newFreshSetup(t)
		e.openSession(t)
		if st := e.status(t); st["session"] != false { // outro navegador (sem cookie) ou sessão expirada
			t.Fatalf("sem cookie, sem sessão: %v", st)
		}
		if rec := do(e.s, "POST", "/api/setup/password", `{"password":"`+ownerPass+`"}`, jsonHdr); rec.Code != 401 {
			t.Fatalf("passo sem sessão = %d", rec.Code)
		}
		expired := &http.Cookie{Name: setupCookie, Value: "abc." + strconv.FormatInt(time.Now().Add(-time.Minute).Unix(), 10) + ".sig"}
		if st := e.status(t, expired); st["session"] != false {
			t.Fatal("cookie vencido ou forjado não vale")
		}
	})

	t.Run("senha salva, canal não escolhido: senha aparece feita, e não entra", func(t *testing.T) {
		e := newFreshSetup(t)
		c := e.openSession(t)
		e.post("/api/setup/password", `{"password":"`+ownerPass+`"}`, c)
		c2 := e.openSession(t) // voltou depois: token de novo
		st := e.status(t, c2)
		if pw := st["password"].(map[string]any); pw["draft"] != true {
			t.Fatalf("senha feita: %v", st)
		}
		if rec := do(e.s, "POST", "/api/login", `{"password":"`+ownerPass+`"}`, jsonHdr); rec.Code != 409 {
			t.Fatalf("senha salva sozinha não libera o login = %d", rec.Code)
		}
	})

	t.Run("bot validado, sem /start: volta no mande /start", func(t *testing.T) {
		e := newFreshSetup(t)
		c := e.openSession(t)
		e.post("/api/setup/password", `{"password":"`+ownerPass+`"}`, c)
		if rec := e.post("/api/setup/telegram/bot", `{"token":"`+botToken+`"}`, c); rec.Code != 200 || !strings.Contains(rec.Body.String(), "meu_bot") {
			t.Fatalf("bot = %d %s", rec.Code, rec.Body)
		}
		if strings.Contains(e.store.draft.TelegramToken, "ABC-bot") {
			t.Fatal("o token do bot fica cifrado no rascunho")
		}
		st := e.status(t, e.openSession(t))
		tg := st["telegram"].(map[string]any)
		if tg["bot"] != "meu_bot" || tg["candidate"] != nil {
			t.Fatalf("esperando /start: %v", tg)
		}
		if rec := e.post("/api/setup/telegram/poll", "{}", e.openSession(t)); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"candidate":null`) {
			t.Fatalf("poll sem mensagem = %d %s", rec.Code, rec.Body)
		}
	})

	t.Run("/start recebido, código não digitado: outro código ou não sou eu", func(t *testing.T) {
		e := newFreshSetup(t)
		c := e.openSession(t)
		e.untilCode(t, c)
		old := e.tg.lastCode(t, ownerID)
		if rec := e.post("/api/setup/telegram/accept", "{}", c); rec.Code != 429 {
			t.Fatalf("reenviar na hora = %d (o intervalo é o mesmo do OTP)", rec.Code)
		}
		if rec := e.post("/api/setup/telegram/reject", "{}", c); rec.Code != 200 {
			t.Fatalf("não sou eu = %d", rec.Code)
		}
		if rec := e.post("/api/setup/telegram/confirm", `{"code":"`+old+`"}`, c); rec.Code != 401 {
			t.Fatalf("o código de quem foi descartado não vale = %d", rec.Code)
		}
	})

	t.Run("fechou na tela Pronto: login normal com senha e código", func(t *testing.T) {
		e := newFreshSetup(t)
		c := e.openSession(t)
		e.untilCode(t, c)
		if rec := e.post("/api/setup/telegram/confirm", `{"code":"`+e.tg.lastCode(t, ownerID)+`"}`, c); rec.Code != 200 {
			t.Fatalf("confirm = %d %s", rec.Code, rec.Body)
		}
		if rec := do(e.s, "GET", "/api/setup/status", "", nil); rec.Code != 404 {
			t.Fatalf("setup concluído: status = %d", rec.Code)
		}
		if rec := do(e.s, "POST", "/api/setup/token", `{"token":"`+setupTok+`"}`, jsonHdr); rec.Code != 404 {
			t.Fatalf("setup concluído: token = %d", rec.Code)
		}
		rec := do(e.s, "POST", "/api/login", `{"password":"`+ownerPass+`"}`, jsonHdr)
		ch := cookieNamed(rec, challengeCookie)
		if rec.Code != 200 || ch == nil || cookieNamed(rec, sessionCookie) != nil {
			t.Fatalf("login pede o código: %d %s", rec.Code, rec.Body)
		}
		if rec := do(e.s, "POST", "/api/login/code", `{"code":"`+e.box.lastCode(t)+`"}`, jsonHdr, ch); rec.Code != 200 || cookieNamed(rec, sessionCookie) == nil {
			t.Fatalf("código = %d %s", rec.Code, rec.Body)
		}
	})
}

// Concluído: canal gravado a partir do rascunho, ativado com o offset depois do /start, sessão do dashboard aberta.
func TestSetup_ConfirmCompletesAndOpensSession(t *testing.T) {
	e := newFreshSetup(t)
	c := e.openSession(t)
	e.untilCode(t, c)
	code := e.tg.lastCode(t, ownerID)
	// Campos a mais na requisição nunca viram configuração (os valores vêm do rascunho confirmado).
	rec := e.post("/api/setup/telegram/confirm", `{"code":"`+code+`","chatId":1,"token":"999:OUTRO"}`, c)
	sess := cookieNamed(rec, sessionCookie)
	if rec.Code != 200 || sess == nil {
		t.Fatalf("confirm = %d %s", rec.Code, rec.Body)
	}
	if cleared := cookieNamed(rec, setupCookie); cleared == nil || cleared.MaxAge >= 0 || len(rec.Result().Header.Values("Set-Cookie")) != 2 {
		t.Fatalf("a sessão de setup é encerrada (um cookie que apaga, mais o da sessão): %v", rec.Result().Header.Values("Set-Cookie"))
	}
	if len(e.activations) != 1 || e.activations[0].ChatID != ownerID || e.activations[0].Token != botToken || e.activations[0].Offset != 101 {
		t.Fatalf("ativação: %+v", e.activations)
	}
	found := map[string]string{}
	for _, r := range e.store.rows {
		found[r.Key] = r.Value
	}
	if found["CHANNEL"] != "telegram" || found["TELEGRAM_CHAT_ID"] != strconv.FormatInt(ownerID, 10) || found["TELEGRAM_BOT_TOKEN"] == botToken {
		t.Fatalf("canal gravado (token cifrado): %v", found)
	}
	if rec := do(e.s, "GET", "/api/me", "", nil, sess); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"secondFactor":true`) {
		t.Fatalf("/api/me depois do setup = %d %s", rec.Code, rec.Body)
	}
	var steps []string
	for _, a := range e.audit.entries {
		if a.Action != "setup" {
			t.Fatalf("ação no histórico: %+v", a)
		}
		steps = append(steps, a.Key)
	}
	if strings.Join(steps, ",") != "SETUP_COMPLETE,TELEGRAM_CHAT_ID,TELEGRAM_BOT_TOKEN,DASHBOARD_PASSWORD,SETUP_TOKEN" {
		t.Fatalf("passos no histórico: %v", steps)
	}
}

// Canal que não liga na hora não desfaz o setup (o app tenta de novo) e não rebaixa o login.
func TestSetup_ActivationFailureKeepsSetupDone(t *testing.T) {
	e := newFreshSetup(t)
	e.activateErr = errors.New("telegram fora")
	c := e.openSession(t)
	e.untilCode(t, c)
	if rec := e.post("/api/setup/telegram/confirm", `{"code":"`+e.tg.lastCode(t, ownerID)+`"}`, c); rec.Code != 200 {
		t.Fatalf("confirm = %d %s", rec.Code, rec.Body)
	}
	if e.svc.Open() {
		t.Fatal("o setup continua concluído")
	}
	e.box.err = errors.New("canal de conversa fora do ar")
	if rec := do(e.s, "POST", "/api/login", `{"password":"`+ownerPass+`"}`, jsonHdr); rec.Code != 503 {
		t.Fatalf("login com o canal fora = %d (falha fechada)", rec.Code)
	}
}

// Duas conclusões ao mesmo tempo: um bot só.
func TestSetup_ConcurrentConfirmActivatesOnce(t *testing.T) {
	e := newFreshSetup(t)
	c := e.openSession(t)
	e.untilCode(t, c)
	code := e.tg.lastCode(t, ownerID)
	var wg sync.WaitGroup
	codes := make([]int, 4)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = e.post("/api/setup/telegram/confirm", `{"code":"`+code+`"}`, c).Code
		}()
	}
	wg.Wait()
	ok := 0
	for _, code := range codes {
		if code == 200 {
			ok++
		}
	}
	if ok != 1 || len(e.activations) != 1 {
		t.Fatalf("respostas %v, ativações %d", codes, len(e.activations))
	}
}

// Outra pessoa manda /start primeiro: aparece o nome dela; "não sou eu" descarta e a espera continua.
func TestSetup_StrangerStartIsRejected(t *testing.T) {
	e := newFreshSetup(t)
	c := e.openSession(t)
	e.post("/api/setup/password", `{"password":"`+ownerPass+`"}`, c)
	e.post("/api/setup/telegram/bot", `{"token":"`+botToken+`"}`, c)
	e.tg.arrive(99, setup.Candidate{ID: 1, Name: "Mensagem Antiga"}) // antes de colar o token: não conta
	e.tg.arrive(100, setup.Candidate{ID: strangerID, Name: "Pessoa Estranha", Username: "estranha"})
	rec := e.post("/api/setup/telegram/poll", "{}", c)
	if !strings.Contains(rec.Body.String(), "Pessoa Estranha") {
		t.Fatalf("poll = %s", rec.Body)
	}
	e.post("/api/setup/telegram/reject", "{}", c)
	e.tg.arrive(101, setup.Candidate{ID: ownerID, Name: "Dono da Conta"})
	if rec := e.post("/api/setup/telegram/poll", "{}", c); !strings.Contains(rec.Body.String(), "Dono da Conta") {
		t.Fatalf("depois do não sou eu = %s", rec.Body)
	}
	e.post("/api/setup/telegram/accept", "{}", c)
	if len(e.tg.sent[strangerID]) != 0 {
		t.Fatal("a pessoa estranha nunca recebe código")
	}
	if rec := e.post("/api/setup/telegram/confirm", `{"code":"`+e.tg.lastCode(t, ownerID)+`"}`, c); rec.Code != 200 || e.activations[0].ChatID != ownerID {
		t.Fatalf("confirm = %d %+v", rec.Code, e.activations)
	}
}

func TestSetup_BotErrors(t *testing.T) {
	e := newFreshSetup(t)
	c := e.openSession(t)
	for body, want := range map[string]int{`{"token":""}`: 400, `{"token":"1:a b"}`: 400, `{"token":"999:ERRADO"}`: 400} {
		if rec := e.post("/api/setup/telegram/bot", body, c); rec.Code != want || !strings.Contains(rec.Body.String(), "token inválido") {
			t.Errorf("%s = %d %s", body, rec.Code, rec.Body)
		}
	}
	e.tg.down = true
	if rec := e.post("/api/setup/telegram/bot", `{"token":"`+botToken+`"}`, c); rec.Code != 502 {
		t.Errorf("telegram fora = %d", rec.Code)
	}
	if rec := e.post("/api/setup/telegram/poll", "{}", c); rec.Code != 409 {
		t.Errorf("poll sem bot = %d", rec.Code)
	}
	if rec := e.post("/api/setup/telegram/confirm", `{"code":"123456"}`, c); rec.Code != 401 {
		t.Errorf("confirm sem código pedido = %d", rec.Code)
	}
}

// Instalação atual com .env completo: nenhuma tela de setup, login como antes.
func TestSetup_ExistingInstallNeverSeesSetup(t *testing.T) {
	e := newSetupEnv(t, setup.Env{Token: setupTok, EnvPassword: testPassword, ChannelReady: func() bool { return true }}, setup.Owner{})
	if rec := do(e.s, "GET", "/api/setup/status", "", nil); rec.Code != 404 {
		t.Fatalf("setup com .env completo = %d", rec.Code)
	}
	e.channelOn.Store(true)
	if rec := do(e.s, "POST", "/api/login", `{"password":"`+testPassword+`"}`, jsonHdr); rec.Code != 200 || cookieNamed(rec, challengeCookie) == nil {
		t.Fatalf("login = %d %s", rec.Code, rec.Body)
	}
}

// Canal no .env, sem senha (API antes desligada): o setup só pede a senha e mantém o canal.
func TestSetup_KeepChannelFromEnv(t *testing.T) {
	e := newSetupEnv(t, setup.Env{Token: setupTok, ChannelReady: func() bool { return true }}, setup.Owner{})
	c := e.openSession(t)
	st := e.status(t, c)
	if ch := st["channel"].(map[string]any); ch["configured"] != true || ch["canReplace"] != false {
		t.Fatalf("canal do .env: %v", st)
	}
	if rec := e.post("/api/setup/telegram/bot", `{"token":"`+botToken+`"}`, c); rec.Code != 409 {
		t.Fatalf("trocar o canal do .env pelo setup = %d", rec.Code)
	}
	if rec := e.post("/api/setup/keep", "{}", c); rec.Code != 409 {
		t.Fatalf("manter sem senha = %d", rec.Code)
	}
	e.post("/api/setup/password", `{"password":"`+ownerPass+`"}`, c)
	var wg sync.WaitGroup
	codes := make([]int, 3)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = e.post("/api/setup/keep", "{}", c).Code
		}()
	}
	wg.Wait()
	if len(e.activations) != 1 || !e.activations[0].Keep {
		t.Fatalf("manter: respostas %v, ativações %+v", codes, e.activations)
	}
}

// SETUP_REOPEN: troca a senha e o canal, mantendo o resto.
func TestSetup_ReopenAllowsReplacingChannel(t *testing.T) {
	done := time.Now()
	e := newSetupEnv(t, setup.Env{Token: setupTok, Reopen: true, ChannelReady: func() bool { return true }}, setup.Owner{PasswordHash: "$argon2id$antiga", CompletedAt: &done})
	if rec := do(e.s, "POST", "/api/login", `{"password":"x"}`, jsonHdr); rec.Code != 409 {
		t.Fatalf("reaberto, o login fica fechado = %d", rec.Code)
	}
	c := e.openSession(t)
	st := e.status(t, c)
	if st["reopen"] != true || st["channel"].(map[string]any)["canReplace"] != true || st["password"].(map[string]any)["current"] != true {
		t.Fatalf("reaberto: %v", st)
	}
	e.untilCode(t, c)
	if rec := e.post("/api/setup/telegram/confirm", `{"code":"`+e.tg.lastCode(t, ownerID)+`"}`, c); rec.Code != 200 {
		t.Fatalf("confirm = %d %s", rec.Code, rec.Body)
	}
	if !e.svc.CheckPassword(ownerPass) || e.svc.Open() {
		t.Fatal("refeito com a senha nova")
	}
}
