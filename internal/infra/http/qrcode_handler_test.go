package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var errQRSentinel = errors.New("qr provider error")

type mockQRProvider struct {
	stateFn func(ctx context.Context) (string, error)
	codeFn  func(ctx context.Context) (string, string, error)
}

func (m *mockQRProvider) FetchConnectionState(ctx context.Context) (string, error) {
	return m.stateFn(ctx)
}

func (m *mockQRProvider) FetchConnectCode(ctx context.Context) (string, string, error) {
	return m.codeFn(ctx)
}

func doQRRequestHeader(h *qrcodeHandler, secret string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/admin/qrcode", nil)
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	rr := httptest.NewRecorder()
	h.Handle(rr, req)
	return rr
}

func doQRRequestQuery(h *qrcodeHandler, secret string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/admin/qrcode?token="+secret, nil)
	rr := httptest.NewRecorder()
	h.Handle(rr, req)
	return rr
}

func TestQRCodeHandler_DisabledWhenNoSecret(t *testing.T) {
	h := &qrcodeHandler{secret: "", qrProvider: &mockQRProvider{}}
	rr := doQRRequestHeader(h, "")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("esperava 503, got %d", rr.Code)
	}
}

func TestQRCodeHandler_UnauthorizedWhenWrongSecret(t *testing.T) {
	h := &qrcodeHandler{secret: "correct-secret", qrProvider: &mockQRProvider{}}

	if rr := doQRRequestHeader(h, "wrong-secret"); rr.Code != http.StatusUnauthorized {
		t.Errorf("header: esperava 401, got %d", rr.Code)
	}
	if rr := doQRRequestQuery(h, "wrong-secret"); rr.Code != http.StatusUnauthorized {
		t.Errorf("query: esperava 401, got %d", rr.Code)
	}
}

func TestQRCodeHandler_AuthorizationHeader(t *testing.T) {
	h := &qrcodeHandler{
		secret: "my-secret",
		qrProvider: &mockQRProvider{
			stateFn: func(_ context.Context) (string, error) { return "open", nil },
		},
	}
	rr := doQRRequestHeader(h, "my-secret")

	if rr.Code != http.StatusOK {
		t.Errorf("esperava 200 com Authorization header, got %d", rr.Code)
	}
}

func TestQRCodeHandler_QueryParamFallback(t *testing.T) {
	h := &qrcodeHandler{
		secret: "my-secret",
		qrProvider: &mockQRProvider{
			stateFn: func(_ context.Context) (string, error) { return "open", nil },
		},
	}
	rr := doQRRequestQuery(h, "my-secret")

	if rr.Code != http.StatusOK {
		t.Errorf("esperava 200 com query param, got %d", rr.Code)
	}
}

func TestQRCodeHandler_AlreadyConnected(t *testing.T) {
	h := &qrcodeHandler{
		secret: "my-secret",
		qrProvider: &mockQRProvider{
			stateFn: func(_ context.Context) (string, error) { return "open", nil },
		},
	}
	rr := doQRRequestHeader(h, "my-secret")

	if rr.Code != http.StatusOK {
		t.Errorf("esperava 200, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("resposta não é JSON: %v", err)
	}
	if body["connected"] != true {
		t.Errorf("esperava connected=true, got %v", body["connected"])
	}
}

func TestQRCodeHandler_ReturnsHTMLWithQR(t *testing.T) {
	const fakeBase64 = "data:image/png;base64,abc123"
	h := &qrcodeHandler{
		secret: "my-secret",
		qrProvider: &mockQRProvider{
			stateFn: func(_ context.Context) (string, error) { return "connecting", nil },
			codeFn:  func(_ context.Context) (string, string, error) { return "code", fakeBase64, nil },
		},
	}
	rr := doQRRequestHeader(h, "my-secret")

	if rr.Code != http.StatusOK {
		t.Errorf("esperava 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("esperava Content-Type text/html, got %q", ct)
	}
	if !strings.Contains(rr.Body.String(), fakeBase64) {
		t.Error("HTML não contém o base64 do QR code")
	}
}

func TestQRCodeHandler_EmptyBase64Returns503(t *testing.T) {
	h := &qrcodeHandler{
		secret: "my-secret",
		qrProvider: &mockQRProvider{
			stateFn: func(_ context.Context) (string, error) { return "connecting", nil },
			codeFn:  func(_ context.Context) (string, string, error) { return "", "", nil },
		},
	}
	if rr := doQRRequestHeader(h, "my-secret"); rr.Code != http.StatusServiceUnavailable {
		t.Errorf("esperava 503 para base64 vazio, got %d", rr.Code)
	}
}

func TestQRCodeHandler_StateErrorReturns500(t *testing.T) {
	h := &qrcodeHandler{
		secret: "my-secret",
		qrProvider: &mockQRProvider{
			stateFn: func(_ context.Context) (string, error) { return "", errQRSentinel },
		},
	}
	if rr := doQRRequestHeader(h, "my-secret"); rr.Code != http.StatusInternalServerError {
		t.Errorf("esperava 500, got %d", rr.Code)
	}
}
