package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWebhookSourceMiddleware_AllowsKnownHost(t *testing.T) {
	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})

	handler := webhookSourceMiddleware("localhost", silentLogger, next)

	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !reached {
		t.Error("esperava que a requisição passasse pelo middleware")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("esperava 200, got %d", rr.Code)
	}
}

func TestWebhookSourceMiddleware_BlocksUnknownIP(t *testing.T) {
	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })

	handler := webhookSourceMiddleware("localhost", silentLogger, next)

	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	req.RemoteAddr = "203.0.113.42:54321"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if reached {
		t.Error("IP externo não deveria passar pelo middleware")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("esperava 403, got %d", rr.Code)
	}
}

func TestWebhookSourceMiddleware_BlocksUnresolvableHost(t *testing.T) {
	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })

	handler := webhookSourceMiddleware("host-que-nao-existe.internal", silentLogger, next)

	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	req.RemoteAddr = "172.20.0.5:54321"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if reached {
		t.Error("host inresolvável deveria bloquear a requisição")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("esperava 403, got %d", rr.Code)
	}
}

func TestWebhookSourceMiddleware_BlocksMalformedRemoteAddr(t *testing.T) {
	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })

	handler := webhookSourceMiddleware("localhost", silentLogger, next)

	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	req.RemoteAddr = "invalid-addr"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if reached {
		t.Error("RemoteAddr malformado deveria bloquear a requisição")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("esperava 403, got %d", rr.Code)
	}
}

func TestIPRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := newIPRateLimiter(3, time.Minute)

	for i := range 3 {
		if !rl.allow("1.2.3.4") {
			t.Errorf("requisição %d deveria ser permitida", i+1)
		}
	}
}

func TestIPRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := newIPRateLimiter(3, time.Minute)

	for range 3 {
		rl.allow("1.2.3.4")
	}
	if rl.allow("1.2.3.4") {
		t.Error("4ª requisição deveria ser bloqueada")
	}
}

func TestIPRateLimiter_IsolatesIPs(t *testing.T) {
	rl := newIPRateLimiter(2, time.Minute)

	rl.allow("1.1.1.1")
	rl.allow("1.1.1.1")

	if !rl.allow("2.2.2.2") {
		t.Error("IP diferente deveria ter seu próprio limite")
	}
}

func TestIPRateLimiter_ResetsAfterWindow(t *testing.T) {
	rl := newIPRateLimiter(2, 50*time.Millisecond)

	rl.allow("1.2.3.4")
	rl.allow("1.2.3.4")

	time.Sleep(60 * time.Millisecond)

	if !rl.allow("1.2.3.4") {
		t.Error("após a janela expirar, requisição deveria ser permitida novamente")
	}
}

func TestAdminRateLimitMiddleware_Returns429WhenLimitExceeded(t *testing.T) {
	rl := newIPRateLimiter(2, time.Minute)
	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ })

	handler := adminRateLimitMiddleware(rl, next)

	for range 3 {
		req := httptest.NewRequest(http.MethodGet, "/admin/qrcode", nil)
		req.RemoteAddr = "5.6.7.8:12345"
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}

	if calls != 2 {
		t.Errorf("esperava 2 chamadas passadas, got %d", calls)
	}
}
