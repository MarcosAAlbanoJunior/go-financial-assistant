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

// apiError é um erro que já sabe o status e a mensagem a devolver ao cliente (validação do corpo, regra de negócio).
// Qualquer outro erro vira 500 genérico em respondErr, sem vazar detalhes internos.
type apiError struct {
	status int
	msg    string
}

func (e *apiError) Error() string { return e.msg }

func badRequest(msg string) *apiError { return &apiError{http.StatusBadRequest, msg} }

// respondErr responde um apiError como ele é e qualquer outro erro como falha interna (registrada no log).
func (a *api) respondErr(w http.ResponseWriter, what string, err error) {
	var ae *apiError
	if errors.As(err, &ae) {
		writeError(w, ae.status, ae.msg)
		return
	}
	a.fail(w, what, err)
}
