package httpserver

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

func TestHandleError_InvalidAmount(t *testing.T) {
	h := newHandler(&mockAnalyzer{}, &mockMessenger{})
	rr := httptest.NewRecorder()
	h.handleError(rr, domain.ErrInvalidAmount)
	if rr.Code != 422 {
		t.Errorf("esperava 422, got %d", rr.Code)
	}
}

func TestHandleError_InvalidPaymentMethod(t *testing.T) {
	h := newHandler(&mockAnalyzer{}, &mockMessenger{})
	rr := httptest.NewRecorder()
	h.handleError(rr, domain.ErrInvalidPaymentMethod)
	if rr.Code != 400 {
		t.Errorf("esperava 400, got %d", rr.Code)
	}
}

func TestHandleError_UnsupportedMessage(t *testing.T) {
	h := newHandler(&mockAnalyzer{}, &mockMessenger{})
	rr := httptest.NewRecorder()
	h.handleError(rr, errUnsupportedMessage)
	if rr.Code != 400 {
		t.Errorf("esperava 400, got %d", rr.Code)
	}
}

func TestHandleError_InvalidImage(t *testing.T) {
	h := newHandler(&mockAnalyzer{}, &mockMessenger{})
	rr := httptest.NewRecorder()
	h.handleError(rr, errInvalidImage)
	if rr.Code != 400 {
		t.Errorf("esperava 400, got %d", rr.Code)
	}
}

func TestHandleError_Default(t *testing.T) {
	h := newHandler(&mockAnalyzer{}, &mockMessenger{})
	rr := httptest.NewRecorder()
	h.handleError(rr, errors.New("erro genérico"))
	if rr.Code != 500 {
		t.Errorf("esperava 500, got %d", rr.Code)
	}
}

func TestNotifyError_StoresSentID(t *testing.T) {
	messenger := &mockMessenger{
		sendTextFn: func(_ context.Context, _, _ string) (string, error) { return "NOTIFY-ID", nil },
	}
	h := newHandler(&mockAnalyzer{}, messenger)
	h.notifyError(context.Background(), errors.New("algo errado"))

	if _, ok := h.sentIDs.Load("NOTIFY-ID"); !ok {
		t.Error("sentID da notificação deveria ter sido armazenado")
	}
}

func TestNotifyError_MessengerError(t *testing.T) {
	messenger := &mockMessenger{
		sendTextFn: func(_ context.Context, _, _ string) (string, error) {
			return "", errors.New("falha ao notificar")
		},
	}
	h := newHandler(&mockAnalyzer{}, messenger)
	h.notifyError(context.Background(), errors.New("erro original"))
}

func TestDecodeBase64Image_WithPrefix(t *testing.T) {
	raw := []byte{1, 2, 3, 4}
	encoded := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(raw)

	got, err := decodeBase64Image(encoded)
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if string(got) != string(raw) {
		t.Error("bytes decodificados incorretos")
	}
}

func TestDecodeBase64Image_WithoutPrefix(t *testing.T) {
	raw := []byte{5, 6, 7}
	got, err := decodeBase64Image(base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if string(got) != string(raw) {
		t.Error("bytes decodificados incorretos")
	}
}

func TestDecodeBase64Image_Invalid(t *testing.T) {
	_, err := decodeBase64Image("!!!not-valid-base64!!!")
	if err == nil {
		t.Fatal("esperava erro de base64 inválido")
	}
}
