package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/ledger"
)

type mockAnalyzer struct {
	executeTextFn     func(ctx context.Context, input ledger.TextInput) (*ledger.ExpenseOutput, error)
	executeImageFn    func(ctx context.Context, input ledger.ImageInput) (*ledger.ExpenseOutput, error)
	executeDocumentFn func(ctx context.Context, input ledger.DocumentInput) (*ledger.StatementOutput, error)
}

func (m *mockAnalyzer) ExecuteText(ctx context.Context, input ledger.TextInput) (*ledger.ExpenseOutput, error) {
	if m.executeTextFn != nil {
		return m.executeTextFn(ctx, input)
	}
	return defaultOutput(), nil
}

func (m *mockAnalyzer) ExecuteImage(ctx context.Context, input ledger.ImageInput) (*ledger.ExpenseOutput, error) {
	return m.executeImageFn(ctx, input)
}

func (m *mockAnalyzer) ExecuteDocument(ctx context.Context, input ledger.DocumentInput) (*ledger.StatementOutput, error) {
	if m.executeDocumentFn != nil {
		return m.executeDocumentFn(ctx, input)
	}
	return &ledger.StatementOutput{}, nil
}

func (m *mockAnalyzer) SavePendingTransaction(ctx context.Context, tx ledger.PendingTransaction) error {
	return nil
}

type mockMessenger struct {
	sendTextFn         func(ctx context.Context, to, text string) (string, error)
	sendDocumentFn     func(ctx context.Context, to, filename string, data []byte, caption string) (string, error)
	fetchImageBase64Fn func(ctx context.Context, remoteJid string, fromMe bool, messageID string) (string, error)
}

func (m *mockMessenger) SendText(ctx context.Context, to, text string) (string, error) {
	if m.sendTextFn != nil {
		return m.sendTextFn(ctx, to, text)
	}
	return "", nil
}

func (m *mockMessenger) SendDocument(ctx context.Context, to, filename string, data []byte, caption string) (string, error) {
	if m.sendDocumentFn != nil {
		return m.sendDocumentFn(ctx, to, filename, data, caption)
	}
	return "", nil
}

func (m *mockMessenger) FetchImageBase64(ctx context.Context, remoteJid string, fromMe bool, messageID string) (string, error) {
	if m.fetchImageBase64Fn != nil {
		return m.fetchImageBase64Fn(ctx, remoteJid, fromMe, messageID)
	}
	return "", nil
}

func (m *mockMessenger) FetchConnectionState(context.Context) (string, error) { return "open", nil }

func (m *mockMessenger) FetchConnectCode(context.Context) (string, string, error) { return "", "", nil }

type mockCSVExporter struct {
	executeFn func(ctx context.Context, month time.Time) ([]byte, string, *ledger.ExportSummary, error)
}

func (m *mockCSVExporter) Execute(ctx context.Context, month time.Time) ([]byte, string, *ledger.ExportSummary, error) {
	if m.executeFn != nil {
		return m.executeFn(ctx, month)
	}
	return nil, "", nil, nil
}

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func defaultOutput() *ledger.ExpenseOutput {
	return &ledger.ExpenseOutput{
		ID: "uuid-1", Amount: 50.0, Description: "Almoço",
		Category: "FOOD", Payment: "PIX", Confidence: 0.95,
	}
}

func newHandler(analyzer ledger.ExpenseAnalyzer, messenger *mockMessenger, exporters ...ledger.CSVExporter) *webhookHandler {
	var exporter ledger.CSVExporter = &mockCSVExporter{}
	if len(exporters) > 0 && exporters[0] != nil {
		exporter = exporters[0]
	}
	return newWebhookHandler(
		WhatsAppConfig{
			OwnerPhone:     "5511999999999",
			AllowedNumbers: map[string]struct{}{"5511888888888@s.whatsapp.net": {}},
		},
		messenger,
		analyzer,
		exporter,
		silentLogger,
	)
}

func buildPayload(instance, remoteJid, msgID string, fromMe bool, msg evolutionMessage, base64Data string) []byte {
	p := evolutionPayload{
		Instance: instance,
		Data: evolutionData{
			Key:     evolutionKey{RemoteJID: remoteJid, FromMe: fromMe, ID: msgID},
			Message: msg,
			Base64:  base64Data,
		},
	}
	b, _ := json.Marshal(p)
	return b
}

func doRequest(h *webhookHandler, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Handle(rr, req)
	return rr
}
