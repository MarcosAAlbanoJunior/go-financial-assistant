package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func okHandler(called *bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { *called = true; w.WriteHeader(http.StatusNoContent) }
}

func TestJSONOnly(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		origin      string
		want        int
	}{
		{"json na mesma origem", "application/json", "", http.StatusNoContent},
		{"json com charset", "application/json; charset=utf-8", "http://example.com", http.StatusNoContent},
		{"formulário (CSRF)", "application/x-www-form-urlencoded", "", http.StatusForbidden},
		{"texto puro", "text/plain", "", http.StatusForbidden},
		{"sem tipo", "", "", http.StatusForbidden},
		{"outra origem", "application/json", "https://evil.example", http.StatusForbidden},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var called bool
			req := httptest.NewRequest("POST", "http://example.com/x", strings.NewReader("{}"))
			if c.contentType != "" {
				req.Header.Set("Content-Type", c.contentType)
			}
			if c.origin != "" {
				req.Header.Set("Origin", c.origin)
			}
			rec := httptest.NewRecorder()
			jsonOnly(okHandler(&called))(rec, req)
			if rec.Code != c.want || called != (c.want == http.StatusNoContent) {
				t.Errorf("status %d, chamou=%v", rec.Code, called)
			}
		})
	}
}

func TestSameOriginOnly(t *testing.T) {
	var called bool
	req := httptest.NewRequest("DELETE", "http://example.com/x", nil)
	rec := httptest.NewRecorder()
	sameOriginOnly(okHandler(&called))(rec, req)
	if rec.Code != http.StatusNoContent || !called {
		t.Errorf("sem Origin passa: %d", rec.Code)
	}

	called = false
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	sameOriginOnly(okHandler(&called))(rec, req)
	if rec.Code != http.StatusForbidden || called {
		t.Errorf("outra origem é barrada: %d", rec.Code)
	}
}

func TestDecodeJSON(t *testing.T) {
	type body struct {
		A int `json:"a"`
	}
	for name, c := range map[string]struct {
		body   string
		strict bool
		ok     bool
	}{
		"válido":                 {`{"a":1}`, false, true},
		"lixo":                   {`nao-json`, false, false},
		"campo extra tolerado":   {`{"a":1,"b":2}`, false, true},
		"campo extra no estrito": {`{"a":1,"b":2}`, true, false},
		"grande demais":          {`{"a":1,"x":"` + strings.Repeat("y", 100) + `"}`, false, false},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/x", strings.NewReader(c.body))
			rec := httptest.NewRecorder()
			var b body
			var got bool
			if c.strict {
				got = decodeJSONStrict(rec, req, 64, &b)
			} else {
				got = decodeJSON(rec, req, 64, &b)
			}
			if got != c.ok {
				t.Fatalf("ok = %v, quer %v", got, c.ok)
			}
			if !c.ok && (rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "corpo inválido")) {
				t.Errorf("deveria responder 400 'corpo inválido': %d %s", rec.Code, rec.Body)
			}
			if c.ok && b.A != 1 {
				t.Errorf("valor não lido: %+v", b)
			}
		})
	}
}

func TestRespondErr(t *testing.T) {
	a := &api{logger: slogDiscard()}
	rec := httptest.NewRecorder()
	a.respondErr(rec, "x", badRequest("campo inválido"))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "campo inválido") {
		t.Errorf("apiError vai como é: %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	a.respondErr(rec, "x", errString("pq: senha do banco vazou"))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "vazou") {
		t.Errorf("erro desconhecido vira 500 genérico: %d %s", rec.Code, rec.Body)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func slogDiscard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
