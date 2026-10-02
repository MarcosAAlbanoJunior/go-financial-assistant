package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
)

// fail registra o erro real no log e devolve ao cliente uma mensagem genérica.
func (a *api) fail(w http.ResponseWriter, what string, err error) {
	if err == nil {
		err = errors.New("resultado inesperado")
	}
	a.logger.Error("erro na API do dashboard", "recurso", what, "error", err)
	writeError(w, http.StatusInternalServerError, "erro interno")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
