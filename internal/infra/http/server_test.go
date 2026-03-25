package httpserver

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

type errReader struct{}

func (e *errReader) Read(_ []byte) (int, error) { return 0, errors.New("read error") }

func TestHandle_InvalidJSON(t *testing.T) {
	h := newHandler(&mockAnalyzer{}, &mockMessenger{})
	rr := doRequest(h, []byte("not json"))
	if rr.Code != 400 {
		t.Errorf("esperava 400, got %d", rr.Code)
	}
}

func TestHandle_InvalidBody(t *testing.T) {
	h := newHandler(&mockAnalyzer{}, &mockMessenger{})
	req := httptest.NewRequest("POST", "/webhook", &errReader{})
	rr := httptest.NewRecorder()
	h.Handle(rr, req)
	if rr.Code != 400 {
		t.Errorf("esperava 400, got %d", rr.Code)
	}
}

func TestHandle_BotMessageDedup(t *testing.T) {
	h := newHandler(&mockAnalyzer{}, &mockMessenger{})
	h.sentIDs.Store("MSG-BOT", struct{}{})

	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-BOT", false,
		evolutionMessage{Conversation: "oi"}, "")
	rr := doRequest(h, body)

	if rr.Code != 200 {
		t.Errorf("esperava 200, got %d", rr.Code)
	}
	if _, still := h.sentIDs.Load("MSG-BOT"); still {
		t.Error("LoadAndDelete deveria ter removido a entrada")
	}
}

func TestHandle_AlreadyProcessed(t *testing.T) {
	h := newHandler(&mockAnalyzer{}, &mockMessenger{})
	body := buildPayload("inst", "desconhecido@s.whatsapp.net", "MSG-DUP", false,
		evolutionMessage{Conversation: "oi"}, "")

	rr1 := doRequest(h, body)
	rr2 := doRequest(h, body)

	if rr1.Code != 200 {
		t.Errorf("primeira: esperava 200, got %d", rr1.Code)
	}
	if rr2.Code != 200 {
		t.Errorf("duplicada: esperava 200, got %d", rr2.Code)
	}
}

func TestHandle_NotAllowedNumber(t *testing.T) {
	h := newHandler(&mockAnalyzer{}, &mockMessenger{})
	body := buildPayload("inst", "55119999999999@s.whatsapp.net", "MSG-X", false,
		evolutionMessage{Conversation: "oi"}, "")
	rr := doRequest(h, body)
	if rr.Code != 200 {
		t.Errorf("esperava 200 (ignorado), got %d", rr.Code)
	}
}

func TestHandle_TextMessage_Success(t *testing.T) {
	analyzer := &mockAnalyzer{
		executeTextFn: func(_ context.Context, _ usecase.TextInput) (*usecase.ExpenseOutput, error) {
			return defaultOutput(), nil
		},
	}
	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-1", false,
		evolutionMessage{Conversation: "gastei 50 pix"}, "")
	rr := doRequest(newHandler(analyzer, &mockMessenger{}), body)
	if rr.Code != 201 {
		t.Errorf("esperava 201, got %d", rr.Code)
	}
}

func TestHandle_TextMessage_AllowedNumber(t *testing.T) {
	analyzer := &mockAnalyzer{
		executeTextFn: func(_ context.Context, _ usecase.TextInput) (*usecase.ExpenseOutput, error) {
			return defaultOutput(), nil
		},
	}
	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-FM", false,
		evolutionMessage{Conversation: "50 pix"}, "")
	rr := doRequest(newHandler(analyzer, &mockMessenger{}), body)
	if rr.Code != 201 {
		t.Errorf("esperava 201, got %d", rr.Code)
	}
}

func TestHandle_ExtendedTextMessage(t *testing.T) {
	var capturedText string
	analyzer := &mockAnalyzer{
		executeTextFn: func(_ context.Context, input usecase.TextInput) (*usecase.ExpenseOutput, error) {
			capturedText = input.Text
			return defaultOutput(), nil
		},
	}
	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-2", false,
		evolutionMessage{ExtendedTextMessage: &evolutionExtendedText{Text: "texto longo"}}, "")
	rr := doRequest(newHandler(analyzer, &mockMessenger{}), body)

	if rr.Code != 201 {
		t.Errorf("esperava 201, got %d", rr.Code)
	}
	if capturedText != "texto longo" {
		t.Errorf("texto esperado 'texto longo', got '%s'", capturedText)
	}
}

func TestHandle_EmptyText_UnsupportedMessage(t *testing.T) {
	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-3", false,
		evolutionMessage{}, "")
	rr := doRequest(newHandler(&mockAnalyzer{}, &mockMessenger{}), body)
	if rr.Code != 400 {
		t.Errorf("esperava 400, got %d", rr.Code)
	}
}

func TestHandle_ImageMessage_WithBase64(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte{1, 2, 3})
	analyzer := &mockAnalyzer{
		executeImageFn: func(_ context.Context, _ usecase.ImageInput) (*usecase.ExpenseOutput, error) {
			return defaultOutput(), nil
		},
	}
	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-IMG", false,
		evolutionMessage{ImageMessage: &evolutionImageMessage{Mimetype: "image/jpeg", Caption: "nota"}}, b64)
	rr := doRequest(newHandler(analyzer, &mockMessenger{}), body)
	if rr.Code != 201 {
		t.Errorf("esperava 201, got %d", rr.Code)
	}
}

func TestHandle_ImageMessage_FetchBase64(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte{4, 5, 6})
	analyzer := &mockAnalyzer{
		executeImageFn: func(_ context.Context, _ usecase.ImageInput) (*usecase.ExpenseOutput, error) {
			return defaultOutput(), nil
		},
	}
	messenger := &mockMessenger{
		fetchImageBase64Fn: func(_ context.Context, _ string, _ bool, _ string) (string, error) {
			return b64, nil
		},
	}
	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-IMG2", false,
		evolutionMessage{ImageMessage: &evolutionImageMessage{Mimetype: "image/png"}}, "")
	rr := doRequest(newHandler(analyzer, messenger), body)
	if rr.Code != 201 {
		t.Errorf("esperava 201, got %d", rr.Code)
	}
}

func TestHandle_ImageMessage_FetchBase64_Error(t *testing.T) {
	messenger := &mockMessenger{
		fetchImageBase64Fn: func(_ context.Context, _ string, _ bool, _ string) (string, error) {
			return "", errors.New("fetch failed")
		},
	}
	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-IMG3", false,
		evolutionMessage{ImageMessage: &evolutionImageMessage{Mimetype: "image/png"}}, "")
	rr := doRequest(newHandler(&mockAnalyzer{}, messenger), body)
	if rr.Code != 400 {
		t.Errorf("esperava 400, got %d", rr.Code)
	}
}

func TestHandle_ImageMessage_InvalidBase64(t *testing.T) {
	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-IMG4", false,
		evolutionMessage{ImageMessage: &evolutionImageMessage{Mimetype: "image/png"}}, "!!!invalid!!!")
	rr := doRequest(newHandler(&mockAnalyzer{}, &mockMessenger{}), body)
	if rr.Code != 400 {
		t.Errorf("esperava 400, got %d", rr.Code)
	}
}

func TestHandle_MessengerSendText_StoresSentID(t *testing.T) {
	analyzer := &mockAnalyzer{
		executeTextFn: func(_ context.Context, _ usecase.TextInput) (*usecase.ExpenseOutput, error) {
			return defaultOutput(), nil
		},
	}
	messenger := &mockMessenger{
		sendTextFn: func(_ context.Context, _, _ string) (string, error) { return "SENT-ID-99", nil },
	}
	h := newHandler(analyzer, messenger)
	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-4", false,
		evolutionMessage{Conversation: "50 pix"}, "")
	doRequest(h, body)

	if _, ok := h.sentIDs.Load("SENT-ID-99"); !ok {
		t.Error("sentID deveria ter sido armazenado")
	}
}

func TestHandle_MessengerSendText_Error(t *testing.T) {
	analyzer := &mockAnalyzer{
		executeTextFn: func(_ context.Context, _ usecase.TextInput) (*usecase.ExpenseOutput, error) {
			return defaultOutput(), nil
		},
	}
	messenger := &mockMessenger{
		sendTextFn: func(_ context.Context, _, _ string) (string, error) {
			return "", errors.New("whatsapp down")
		},
	}
	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-5", false,
		evolutionMessage{Conversation: "50 pix"}, "")
	rr := doRequest(newHandler(analyzer, messenger), body)
	if rr.Code != 201 {
		t.Errorf("erro no messenger não deve afetar resposta HTTP, got %d", rr.Code)
	}
}
