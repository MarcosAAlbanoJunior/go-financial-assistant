package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

type mockMessengerUC struct {
	sendDocumentFn func(ctx context.Context, to, filename, base64Data, caption string) (string, error)
	sendTextFn     func(ctx context.Context, to, text string) (string, error)
}

func (m *mockMessengerUC) SendText(ctx context.Context, to, text string) (string, error) {
	if m.sendTextFn != nil {
		return m.sendTextFn(ctx, to, text)
	}
	return "", nil
}

func (m *mockMessengerUC) SendDocument(ctx context.Context, to, filename, base64Data, caption string) (string, error) {
	if m.sendDocumentFn != nil {
		return m.sendDocumentFn(ctx, to, filename, base64Data, caption)
	}
	return "", nil
}

func (m *mockMessengerUC) FetchImageBase64(ctx context.Context, remoteJid string, fromMe bool, messageID string) (string, error) {
	return "", nil
}

type mockCSVExporterUC struct {
	executeFn func(ctx context.Context, month time.Time) ([]byte, string, *ExportSummary, error)
}

func (m *mockCSVExporterUC) Execute(ctx context.Context, month time.Time) ([]byte, string, *ExportSummary, error) {
	if m.executeFn != nil {
		return m.executeFn(ctx, month)
	}
	return nil, "", nil, nil
}

func silentReportLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestMonthlyReport_SendsDocument(t *testing.T) {
	documentSent := false
	messenger := &mockMessengerUC{
		sendDocumentFn: func(_ context.Context, _, _, _, _ string) (string, error) {
			documentSent = true
			return "msg-id", nil
		},
	}
	exporter := &mockCSVExporterUC{
		executeFn: func(_ context.Context, _ time.Time) ([]byte, string, *ExportSummary, error) {
			return []byte("csv content"), "despesas_fevereiro_2025.csv", nil, nil
		},
	}

	r := NewMonthlyReport(exporter, messenger, "5511999999999", silentReportLogger())
	if err := r.Send(context.Background()); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !documentSent {
		t.Error("SendDocument não foi chamado")
	}
}

func TestMonthlyReport_EmptyMonth_DoesNotSend(t *testing.T) {
	documentSent := false
	messenger := &mockMessengerUC{
		sendDocumentFn: func(_ context.Context, _, _, _, _ string) (string, error) {
			documentSent = true
			return "", nil
		},
	}
	exporter := &mockCSVExporterUC{
		executeFn: func(_ context.Context, _ time.Time) ([]byte, string, *ExportSummary, error) {
			return nil, "", nil, nil
		},
	}

	r := NewMonthlyReport(exporter, messenger, "5511999999999", silentReportLogger())
	if err := r.Send(context.Background()); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if documentSent {
		t.Error("SendDocument não deve ser chamado quando não há despesas")
	}
}

func TestMonthlyReport_ExporterError_ReturnsError(t *testing.T) {
	exporter := &mockCSVExporterUC{
		executeFn: func(_ context.Context, _ time.Time) ([]byte, string, *ExportSummary, error) {
			return nil, "", nil, errors.New("db error")
		},
	}

	r := NewMonthlyReport(exporter, &mockMessengerUC{}, "5511999999999", silentReportLogger())
	if err := r.Send(context.Background()); err == nil {
		t.Error("esperava erro do exporter")
	}
}

func TestMonthlyReport_MessengerError_ReturnsError(t *testing.T) {
	messenger := &mockMessengerUC{
		sendDocumentFn: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "", errors.New("whatsapp error")
		},
	}
	exporter := &mockCSVExporterUC{
		executeFn: func(_ context.Context, _ time.Time) ([]byte, string, *ExportSummary, error) {
			return []byte("csv"), "file.csv", nil, nil
		},
	}

	r := NewMonthlyReport(exporter, messenger, "5511999999999", silentReportLogger())
	if err := r.Send(context.Background()); err == nil {
		t.Error("esperava erro do messenger")
	}
}

func TestMonthlyReport_UsesCorrectPhone(t *testing.T) {
	phoneSent := ""
	messenger := &mockMessengerUC{
		sendDocumentFn: func(_ context.Context, to, _, _, _ string) (string, error) {
			phoneSent = to
			return "", nil
		},
	}
	exporter := &mockCSVExporterUC{
		executeFn: func(_ context.Context, _ time.Time) ([]byte, string, *ExportSummary, error) {
			return []byte("csv"), "file.csv", nil, nil
		},
	}

	r := NewMonthlyReport(exporter, messenger, "5511888888888", silentReportLogger())
	r.Send(context.Background()) //nolint:errcheck

	if phoneSent != "5511888888888" {
		t.Errorf("phone: esperava 5511888888888, got %q", phoneSent)
	}
}
