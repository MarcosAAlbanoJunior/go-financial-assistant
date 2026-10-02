package httpserver

import (
	"encoding/json"
	"mime"
	"net/http"
)

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	// JSON obrigatório + mesma origem: um formulário de outro site não consegue fazer login em nome do usuário (CSRF).
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" || !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "requisição não permitida")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBody)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
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
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "requisição não permitida")
		return
	}
	a.sessions.clear(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *api) registerAuth(rt routes, loginLimiter *ipRateLimiter) {
	rt.mux.Handle("POST /api/login", loginLimiter.middleware(http.HandlerFunc(a.login)))
	rt.mux.Handle("POST /api/logout", rt.protected(a.logout))
	rt.mux.Handle("GET /api/me", rt.protected(func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, map[string]bool{"ok": true}) }))
}
