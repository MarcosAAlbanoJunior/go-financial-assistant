package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/auth"
)

const (
	challengeCookie = "fa_challenge"
	sendCodeTimeout = 15 * time.Second
)

// SecondFactor liga o código no chat ao login e à confirmação das configurações sensíveis.
type SecondFactor struct {
	Enabled bool // DASHBOARD_2FA (auto = true)
	// Send manda o código ao chat da pessoa e diz se conseguiu. Fica nil enquanto não há canal; sem ele, vale só a senha.
	Send func(ctx context.Context, text string) error
}

// Active diz se o login e as confirmações pedem o código.
func (f *SecondFactor) Active() bool { return f != nil && f.Enabled && f.Send != nil }

// SetSecondFactor liga o segundo fator à API (chame antes de MountAPI; Send pode ser preenchido depois, antes de Start).
func (s *Server) SetSecondFactor(f *SecondFactor) { s.factor = f }

// sendCode cria o desafio e manda o código ao chat. Se o envio falhar, o desafio é descartado e ninguém entra (falha fechada).
func (a *api) sendCode(w http.ResponseWriter, r *http.Request, kind auth.Kind, bind string, message func(code string) string) bool {
	code, err := a.challenges.Start(kind, bind)
	switch {
	case errors.Is(err, auth.ErrTooSoon), errors.Is(err, auth.ErrSendLimitReached):
		writeError(w, http.StatusTooManyRequests, err.Error())
		return false
	case err != nil:
		a.fail(w, "gerar código", err)
		return false
	}
	ctx, cancel := context.WithTimeout(r.Context(), sendCodeTimeout)
	defer cancel()
	if err := a.factor.Send(ctx, message(code)); err != nil {
		a.challenges.Cancel(kind)
		a.logger.Error("erro ao enviar o código ao chat", "error", err)
		writeError(w, http.StatusServiceUnavailable, "não foi possível enviar o código ao chat")
		return false
	}
	return true
}

// codeRejected responde a um código recusado. Ao esgotar as tentativas, avisa no chat (no máximo uma vez por janela).
func (a *api) codeRejected(w http.ResponseWriter, r *http.Request, err error, status int, where string) {
	a.logger.Warn("código recusado", "onde", where, "ip", clientIP(r))
	if errors.Is(err, auth.ErrTooManyAttempts) {
		a.noticeOnce("⚠️ " + strconv.Itoa(auth.MaxAttempts) + " códigos errados " + where + " do dashboard (" + time.Now().Format("02/01 15:04") + "). Se não foi você, troque a DASHBOARD_PASSWORD e reinicie o app.")
	}
	writeError(w, status, err.Error())
}

// noticeOnce avisa no chat, no máximo uma vez por failedNoticeEvery, para tentativas erradas não virarem spam.
func (a *api) noticeOnce(text string) {
	if n := time.Now().UnixNano(); n-a.lastFailNotice.Load() > int64(failedNoticeEvery) {
		a.lastFailNotice.Store(n)
		a.notify(context.Background(), text)
	}
}

func setChallengeCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: challengeCookie, Value: value, Path: "/api/login", MaxAge: maxAge,
		HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteStrictMode,
	})
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func loginCodeMessage(code string) string {
	return "🔑 Código de acesso ao dashboard: " + code + "\nVale 5 minutos. Se não foi você que tentou entrar, alguém sabe a sua senha: troque a DASHBOARD_PASSWORD."
}

func confirmCodeMessage(code string) string {
	return "🔑 Código para confirmar a alteração nas Configurações: " + code + "\nVale 5 minutos. Se não foi você, alguém está com uma sessão aberta no seu dashboard: troque a DASHBOARD_PASSWORD e reinicie o app."
}
