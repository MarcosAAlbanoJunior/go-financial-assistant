package httpserver

import (
	"encoding/json"
	"mime"
	"net/http"
)

// jsonOnly protege as escritas: só passa corpo JSON vindo da mesma origem. Um formulário de outro site não consegue
// enviar `application/json` nem passa na checagem de origem, o que barra CSRF sem precisar de token.
func jsonOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" || !sameOrigin(r) {
			writeError(w, http.StatusForbidden, "requisição não permitida")
			return
		}
		next(w, r)
	}
}

// sameOriginOnly é o jsonOnly das escritas sem corpo (logout, DELETE).
func sameOriginOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			writeError(w, http.StatusForbidden, "requisição não permitida")
			return
		}
		next(w, r)
	}
}

// decodeJSON lê o corpo (limitado a max bytes) em v. Se não for um JSON válido, já responde 400 e devolve false.
func decodeJSON(w http.ResponseWriter, r *http.Request, max int64, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, max)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return false
	}
	return true
}

// decodeJSONStrict é o decodeJSON que também recusa campos desconhecidos.
func decodeJSONStrict(w http.ResponseWriter, r *http.Request, max int64, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, max))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return false
	}
	return true
}
