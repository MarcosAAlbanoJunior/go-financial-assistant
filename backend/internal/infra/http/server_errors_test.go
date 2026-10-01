package httpserver

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/chat"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
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
	h.handleError(rr, chat.ErrUnsupportedMessage)
	if rr.Code != 400 {
		t.Errorf("esperava 400, got %d", rr.Code)
	}
}

func TestHandleError_InvalidImage(t *testing.T) {
	h := newHandler(&mockAnalyzer{}, &mockMessenger{})
	rr := httptest.NewRecorder()
	h.handleError(rr, chat.ErrInvalidImage)
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

func TestHandle_ErrorNotification_StoresSentID(t *testing.T) {
	analyzer := &mockAnalyzer{
		executeTextFn: func(_ context.Context, _ usecase.TextInput) (*usecase.ExpenseOutput, error) {
			return nil, errors.New("algo errado")
		},
	}
	messenger := &mockMessenger{
		sendTextFn: func(_ context.Context, _, _ string) (string, error) { return "NOTIFY-ID", nil },
	}
	h := newHandler(analyzer, messenger)
	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-ERR", false,
		evolutionMessage{Conversation: "50 pix"}, "")
	rr := doRequest(h, body)

	if rr.Code != 500 {
		t.Errorf("esperava 500, got %d", rr.Code)
	}
	if _, ok := h.sentIDs.Load("NOTIFY-ID"); !ok {
		t.Error("sentID da notificação deveria ter sido armazenado")
	}
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
