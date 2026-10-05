package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/auth"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/setup"
)

const (
	maxSetupBody    = 2 << 10
	setupTGTimeout  = 15 * time.Second
	maxBotTokenLen  = 200
	setupNotFound   = "setup concluído"
	setupRequiredMs = "configuração inicial pendente"
)

// SetupTelegram fala com o Telegram durante o setup, antes de o bot subir. O token vem do rascunho a cada chamada.
// Os erros já são os do setup (setup.ErrBotTokenInvalid, ErrBotBusy, ErrTelegramDown).
type SetupTelegram interface {
	Bot(ctx context.Context, token string) (username string, err error)
	LatestOffset(ctx context.Context, token string) (int64, error)
	FirstPrivate(ctx context.Context, token string, offset int64) (*setup.Candidate, int64, error)
	Send(ctx context.Context, token string, chatID int64, text string) error
}

// SetupDeps é o que o setup pelo navegador precisa.
type SetupDeps struct {
	Service  *setup.Service
	Telegram SetupTelegram
	// Activate liga, sem reiniciar, o canal que a conclusão acabou de gravar (ou o canal mantido). Um erro aqui não desfaz o
	// setup: ele já está concluído e o app tenta ligar o canal de novo sozinho.
	Activate func(ctx context.Context, a setup.Activation) error
	Audit    settings.AuditLog
}

// SetSetup liga o setup pelo navegador (chame antes de MountAPI). Com o setup aberto, o resto de /api responde 409.
func (s *Server) SetSetup(d *SetupDeps) { s.setup = d }

// setupGate: com o setup aberto, a API do dashboard (inclusive o login) não atende; só /api/setup/*. O front leva para
// a tela do setup quando recebe {"setup": "required"}.
func (s *Server) setupGate(w http.ResponseWriter, r *http.Request) bool {
	p := r.URL.Path
	if s.setup == nil || !strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/api/setup/") || !s.setup.Service.Open() {
		return false
	}
	writeJSON(w, http.StatusConflict, map[string]string{"error": setupRequiredMs, "setup": "required"})
	return true
}

func (a *api) registerSetup(rt routes) {
	tokenLimiter := newIPRateLimiter(5, time.Minute) // 5 tentativas de token por minuto por IP (certas ou erradas)
	stepLimiter := newIPRateLimiter(120, time.Minute)
	open := func(h http.HandlerFunc) http.Handler { return a.setupOpenOnly(stepLimiter.middleware(h)) }
	step := func(h func(http.ResponseWriter, *http.Request, string)) http.Handler {
		return open(jsonOnly(a.setupSess.require(h)))
	}
	rt.mux.Handle("GET /api/setup/status", open(a.setupStatus))
	rt.mux.Handle("POST /api/setup/token", a.setupOpenOnly(tokenLimiter.middleware(http.HandlerFunc(jsonOnly(a.setupToken)))))
	rt.mux.Handle("POST /api/setup/password", step(a.setupPassword))
	rt.mux.Handle("POST /api/setup/telegram/bot", step(a.setupBot))
	rt.mux.Handle("POST /api/setup/telegram/poll", step(a.setupPoll))
	rt.mux.Handle("POST /api/setup/telegram/reject", step(a.setupReject))
	rt.mux.Handle("POST /api/setup/telegram/accept", step(a.setupAccept))
	rt.mux.Handle("POST /api/setup/telegram/confirm", step(a.setupConfirm))
	rt.mux.Handle("POST /api/setup/keep", step(a.setupKeep))
}

// setupOpenOnly: setup concluído, as rotas somem (404), inclusive para quem ainda tem o token.
func (a *api) setupOpenOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.setup.Service.Open() {
			writeError(w, http.StatusNotFound, setupNotFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// setupStatus diz em que pé o setup está. Sem sessão de setup, só o que é preciso para a tela do token (nada dos passos).
func (a *api) setupStatus(w http.ResponseWriter, r *http.Request) {
	svc := a.setup.Service
	resp := map[string]any{
		"open": true, "reopen": svc.Reopen(), "session": false,
		"tokenMinLength": setup.MinTokenLen, "passwordMinLength": setup.MinPasswordLen,
	}
	if err := svc.TokenProblem(); err != nil {
		resp["problem"], resp["problemKind"] = err.Error(), problemKind(err)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if a.setupSess.id(r) == "" {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	st, err := svc.Status(r.Context())
	if err != nil {
		a.fail(w, "estado do setup", err)
		return
	}
	resp["session"] = true
	resp["password"] = map[string]bool{"draft": st.PasswordDraft, "fromEnv": st.PasswordFromEnv, "current": st.PasswordCurrent}
	// Fora da reabertura, um canal que já existe não é trocado (seriam dois bots): só dá para mantê-lo.
	resp["channel"] = map[string]bool{"configured": st.ChannelInEnv, "canReplace": st.Reopen || !st.ChannelInEnv}
	resp["telegram"] = telegramJSON(st.Draft)
	writeJSON(w, http.StatusOK, resp)
}

func problemKind(err error) string {
	switch {
	case errors.Is(err, setup.ErrNoToken):
		return "no-token"
	case errors.Is(err, setup.ErrTokenTooShort):
		return "short-token"
	case errors.Is(err, setup.ErrNoEncryption):
		return "no-key"
	}
	return "database"
}

type candidateJSON struct {
	Name     string `json:"name"`
	Username string `json:"username"`
}

func telegramJSON(d setup.Draft) map[string]any {
	var c *candidateJSON
	if d.Candidate != nil {
		c = &candidateJSON{Name: d.Candidate.Name, Username: d.Candidate.Username}
	}
	return map[string]any{"bot": d.TelegramBot, "candidate": c, "accepted": d.CandidateAccepted}
}

// setupToken confere o SETUP_TOKEN (colado num campo, nunca na URL) e abre a sessão de setup.
func (a *api) setupToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, maxSetupBody, &body) {
		return
	}
	err := a.setup.Service.CheckToken(strings.TrimSpace(body.Token))
	switch {
	case errors.Is(err, setup.ErrWrongToken):
		a.logger.Warn("token de setup recusado", "ip", clientIP(r))
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	case err != nil:
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err := a.setupSess.open(w, r); err != nil {
		a.fail(w, "sessão de setup", err)
		return
	}
	a.setupRecord(r, "SETUP_TOKEN")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *api) setupPassword(w http.ResponseWriter, r *http.Request, _ string) {
	var body struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, maxSetupBody, &body) {
		return
	}
	if err := a.setup.Service.SetPassword(r.Context(), body.Password); err != nil {
		a.setupErr(w, "senha do setup", err)
		return
	}
	a.setupRecord(r, "DASHBOARD_PASSWORD")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// setupBot confere o token do bot (getMe) e guarda no rascunho, já marcando de onde começa a espera do /start: só vale
// mensagem que chegar depois daqui.
func (a *api) setupBot(w http.ResponseWriter, r *http.Request, _ string) {
	var body struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, maxSetupBody, &body) {
		return
	}
	token := strings.TrimSpace(body.Token)
	if token == "" || len(token) > maxBotTokenLen || strings.ContainsAny(token, " \t\r\n/?#") {
		writeError(w, http.StatusBadRequest, setup.ErrBotTokenInvalid.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), setupTGTimeout)
	defer cancel()
	name, err := a.setup.Telegram.Bot(ctx, token)
	if err != nil {
		a.setupErr(w, "token do bot", err)
		return
	}
	offset, err := a.setup.Telegram.LatestOffset(ctx, token)
	if err != nil {
		a.setupErr(w, "mensagens do bot", err)
		return
	}
	if err := a.setup.Service.SetBot(r.Context(), token, name, offset); err != nil {
		a.setupErr(w, "token do bot", err)
		return
	}
	a.setupRecord(r, "TELEGRAM_BOT_TOKEN")
	writeJSON(w, http.StatusOK, map[string]string{"bot": name})
}

// setupPoll olha (sem esperar) se chegou a primeira mensagem privada ao bot. A tela chama a cada 2 s.
func (a *api) setupPoll(w http.ResponseWriter, r *http.Request, _ string) {
	// Concluir espera as consultas em andamento: o bot só começa a ler o Telegram depois que a espera do setup parou.
	a.setupPollMu.RLock()
	defer a.setupPollMu.RUnlock()
	svc := a.setup.Service
	st, err := svc.Status(r.Context())
	if err != nil {
		a.setupErr(w, "estado do setup", err)
		return
	}
	d := st.Draft
	if d.TelegramToken == "" {
		a.setupErr(w, "espera do /start", setup.ErrNoBot)
		return
	}
	if d.Candidate == nil {
		ctx, cancel := context.WithTimeout(r.Context(), setupTGTimeout)
		defer cancel()
		c, next, err := a.setup.Telegram.FirstPrivate(ctx, d.TelegramToken, d.TelegramOffset)
		switch {
		case err != nil:
			a.setupErr(w, "espera do /start", err)
			return
		case c != nil:
			d, err = svc.SetCandidate(r.Context(), d.TelegramToken, *c, next)
		case next != d.TelegramOffset:
			err = svc.AdvanceOffset(r.Context(), d.TelegramToken, next)
		}
		if err != nil {
			a.setupErr(w, "espera do /start", err)
			return
		}
	}
	writeJSON(w, http.StatusOK, telegramJSON(d))
}

// setupReject é o "não sou eu": descarta quem mandou (e o código, se já tinha ido) e continua esperando.
func (a *api) setupReject(w http.ResponseWriter, r *http.Request, _ string) {
	if err := a.setup.Service.RejectCandidate(r.Context()); err != nil {
		a.setupErr(w, "não sou eu", err)
		return
	}
	a.challenges.Cancel(auth.Setup)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// setupAccept é o "sim, sou eu": manda um código ao chat que mandou /start (também serve para reenviar).
func (a *api) setupAccept(w http.ResponseWriter, r *http.Request, id string) {
	d, err := a.setup.Service.AcceptCandidate(r.Context())
	if err != nil {
		a.setupErr(w, "sou eu", err)
		return
	}
	send := func(ctx context.Context, text string) error {
		return a.setup.Telegram.Send(ctx, d.TelegramToken, d.Candidate.ID, text)
	}
	if a.sendCode(w, r, auth.Setup, id, setupCodeMessage, send) {
		writeJSON(w, http.StatusOK, map[string]string{"step": "code"})
	}
}

// setupConfirm confere o código e conclui: grava o canal e a senha (transação), para a espera do /start, liga o canal e
// abre a sessão do dashboard. Nada da requisição vira configuração: o canal vem do rascunho já confirmado.
func (a *api) setupConfirm(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, maxSetupBody, &body) {
		return
	}
	a.setupPollMu.Lock()
	defer a.setupPollMu.Unlock()
	if err := a.challenges.Verify(auth.Setup, id, strings.TrimSpace(body.Code)); err != nil {
		a.codeRejected(w, r, err, http.StatusUnauthorized, "no setup")
		return
	}
	a.setupRecord(r, "TELEGRAM_CHAT_ID")
	act, err := a.setup.Service.CompleteTelegram(r.Context())
	if err != nil {
		a.setupErr(w, "concluir o setup", err)
		return
	}
	a.finishSetup(w, r, act)
}

// setupKeep conclui mantendo o canal que já está configurado (no .env, ou o de antes na reabertura).
func (a *api) setupKeep(w http.ResponseWriter, r *http.Request, _ string) {
	a.setupPollMu.Lock()
	defer a.setupPollMu.Unlock()
	if err := a.setup.Service.CompleteKeep(r.Context()); err != nil {
		a.setupErr(w, "concluir o setup", err)
		return
	}
	a.finishSetup(w, r, setup.Activation{Keep: true})
}

func (a *api) finishSetup(w http.ResponseWriter, r *http.Request, act setup.Activation) {
	if err := a.setup.Activate(r.Context(), act); err != nil {
		a.logger.Error("setup concluído, mas o canal não ligou agora; o app tenta de novo sozinho", "error", err)
	}
	a.logger.Info("setup concluído pelo navegador", "ip", clientIP(r))
	a.setupRecord(r, "SETUP_COMPLETE")
	a.setupSess.clear(w, r)
	// A pessoa acabou de provar o token, a senha e o chat: entra direto.
	a.sessions.issue(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// setupErr responde os erros do setup com a mensagem pronta para a tela; o resto vira 500 genérico.
func (a *api) setupErr(w http.ResponseWriter, what string, err error) {
	switch {
	case errors.Is(err, setup.ErrClosed):
		writeError(w, http.StatusNotFound, setupNotFound)
	case errors.Is(err, setup.ErrPasswordShort), errors.Is(err, setup.ErrBotTokenInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, setup.ErrTelegramDown):
		writeError(w, http.StatusBadGateway, err.Error())
	case errors.Is(err, setup.ErrAlreadyComplete), errors.Is(err, setup.ErrNoPassword), errors.Is(err, setup.ErrNoBot),
		errors.Is(err, setup.ErrNoCandidate), errors.Is(err, setup.ErrNotAccepted), errors.Is(err, setup.ErrNoChannel),
		errors.Is(err, setup.ErrChannelInEnv), errors.Is(err, setup.ErrNoEncryption), errors.Is(err, setup.ErrBotBusy):
		writeError(w, http.StatusConflict, err.Error())
	default:
		a.fail(w, what, err)
	}
}

// setupRecord guarda cada passo no histórico de alterações (sem valores).
func (a *api) setupRecord(r *http.Request, step string) {
	if a.setup.Audit == nil {
		return
	}
	if err := a.setup.Audit.RecordAudit(r.Context(), settings.AuditEntry{Action: "setup", Key: step, Sensitive: true, IP: clientIP(r)}); err != nil {
		a.logger.Error("erro ao registrar passo do setup", "error", err)
	}
}

// setupStepLabels são os nomes dos passos do setup no histórico de alterações.
var setupStepLabels = map[string]string{
	"SETUP_TOKEN":        "Setup: token aceito",
	"DASHBOARD_PASSWORD": "Setup: senha do dashboard definida",
	"TELEGRAM_BOT_TOKEN": "Setup: bot do Telegram conferido",
	"TELEGRAM_CHAT_ID":   "Setup: chat confirmado com o código",
	"SETUP_COMPLETE":     "Setup concluído",
}

func setupCodeMessage(code string) string {
	return "🔑 Código para confirmar este chat no setup do FinAssist: " + code + "\nVale 5 minutos. Se não foi você que está configurando o app, ignore esta mensagem."
}
