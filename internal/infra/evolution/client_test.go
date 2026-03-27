package evolution

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(server *httptest.Server) *Client {
	return &Client{
		baseURL:    server.URL,
		instance:   "test-instance",
		apiKey:     "test-key",
		httpClient: server.Client(),
	}
}

func TestSendText_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"key": map[string]string{"id": "MSG-123"},
		})
	}))
	defer srv.Close()

	id, err := newTestClient(srv).SendText(context.Background(), "5511999@s.whatsapp.net", "oi")
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if id != "MSG-123" {
		t.Errorf("id esperado 'MSG-123', got '%s'", id)
	}
}

func TestSendText_StripAtSign(t *testing.T) {
	var capturedBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		json.NewEncoder(w).Encode(map[string]any{"key": map[string]string{"id": "x"}})
	}))
	defer srv.Close()

	newTestClient(srv).SendText(context.Background(), "5511999@s.whatsapp.net", "texto")

	if capturedBody["number"] != "5511999" {
		t.Errorf("esperava número sem @, got '%s'", capturedBody["number"])
	}
}

func TestSendText_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).SendText(context.Background(), "5511999", "oi")
	if err == nil {
		t.Fatal("esperava erro para status 4xx")
	}
}

func TestSendText_InvalidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("não é json"))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).SendText(context.Background(), "5511999", "oi")
	if err == nil {
		t.Fatal("esperava erro para JSON inválido na resposta")
	}
}

func TestSendText_HeadersAndEndpoint(t *testing.T) {
	var capturedAPIKey, capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAPIKey = r.Header.Get("apikey")
		capturedPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]any{"key": map[string]string{"id": ""}})
	}))
	defer srv.Close()

	newTestClient(srv).SendText(context.Background(), "5511999", "oi")

	if capturedAPIKey != "test-key" {
		t.Errorf("apikey esperada 'test-key', got '%s'", capturedAPIKey)
	}
	if capturedPath != "/message/sendText/test-instance" {
		t.Errorf("path incorreto: %s", capturedPath)
	}
}

func TestFetchImageBase64_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"base64": "abc123=="})
	}))
	defer srv.Close()

	result, err := newTestClient(srv).FetchImageBase64(context.Background(), "jid", true, "MSG-1")
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if result != "abc123==" {
		t.Errorf("base64 esperado 'abc123==', got '%s'", result)
	}
}

func TestFetchImageBase64_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).FetchImageBase64(context.Background(), "jid", false, "MSG-2")
	if err == nil {
		t.Fatal("esperava erro para status 4xx")
	}
}

func TestFetchImageBase64_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("não é json"))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).FetchImageBase64(context.Background(), "jid", false, "MSG-3")
	if err == nil {
		t.Fatal("esperava erro para JSON inválido")
	}
}

func TestFetchImageBase64_HeadersAndEndpoint(t *testing.T) {
	var capturedAPIKey, capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAPIKey = r.Header.Get("apikey")
		capturedPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]string{"base64": ""})
	}))
	defer srv.Close()

	newTestClient(srv).FetchImageBase64(context.Background(), "jid", false, "MSG-4")

	if capturedAPIKey != "test-key" {
		t.Errorf("apikey esperada 'test-key', got '%s'", capturedAPIKey)
	}
	if capturedPath != "/chat/getBase64FromMediaMessage/test-instance" {
		t.Errorf("path incorreto: %s", capturedPath)
	}
}
