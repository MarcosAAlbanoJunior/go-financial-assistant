package httpserver

import (
	"net/http"
)

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	// JSON obrigatório + mesma origem: um formulário de outro site não consegue fazer login em nome do usuário (CSRF).
	var body struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, maxLoginBody, &body) {
		return
	}
	if !a.sessions.checkPassword(body.Password) {
		a.logger.Warn("login do dashboard recusado", "ip", clientIP(r))
		writeError(w, http.StatusUnauthorized, "senha incorreta")
		return
	}
	a.sessions.issue(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *api) logout(w http.ResponseWriter, r *http.Request) {
	a.sessions.clear(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *api) registerAuth(rt routes, loginLimiter *ipRateLimiter) {
	rt.mux.Handle("POST /api/login", loginLimiter.middleware(http.HandlerFunc(jsonOnly(a.login))))
	rt.mux.Handle("POST /api/logout", rt.protected(sameOriginOnly(a.logout)))
	rt.mux.Handle("GET /api/me", rt.protected(func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, map[string]bool{"ok": true}) }))
}
