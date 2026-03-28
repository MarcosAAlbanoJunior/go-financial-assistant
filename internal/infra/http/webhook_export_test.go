package httpserver

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

func exportAnalyzer(month time.Time) *mockAnalyzer {
	return &mockAnalyzer{
		executeTextFn: func(_ context.Context, _ usecase.TextInput) (*usecase.ExpenseOutput, error) {
			return &usecase.ExpenseOutput{Type: "EXPORT_CSV", ExportMonthTime: month}, nil
		},
	}
}

func TestHandleExportCommand_SendsDocument(t *testing.T) {
	documentSent := false
	messenger := &mockMessenger{
		sendDocumentFn: func(_ context.Context, _, _, _, _ string) (string, error) {
			documentSent = true
			return "msg-id-123", nil
		},
	}
	exporter := &mockCSVExporter{
		executeFn: func(_ context.Context, _ time.Time) ([]byte, string, error) {
			return []byte("csv data"), "despesas_marco_2025.csv", nil
		},
	}

	month := time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC)
	h := newHandler(exportAnalyzer(month), messenger, exporter)
	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-EXP-1", false,
		evolutionMessage{Conversation: "exportar março 2025"}, "")
	rr := doRequest(h, body)

	if rr.Code != http.StatusOK {
		t.Errorf("esperava 200, got %d", rr.Code)
	}
	if !documentSent {
		t.Error("SendDocument não foi chamado")
	}
}

func TestHandleExportCommand_EmptyMonth_SendsText(t *testing.T) {
	textSent := ""
	messenger := &mockMessenger{
		sendTextFn: func(_ context.Context, _, text string) (string, error) {
			textSent = text
			return "", nil
		},
	}
	exporter := &mockCSVExporter{
		executeFn: func(_ context.Context, _ time.Time) ([]byte, string, error) {
			return nil, "", nil
		},
	}

	month := time.Date(2020, time.February, 1, 0, 0, 0, 0, time.UTC)
	h := newHandler(exportAnalyzer(month), messenger, exporter)
	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-EXP-2", false,
		evolutionMessage{Conversation: "exportar fevereiro 2020"}, "")
	rr := doRequest(h, body)

	if rr.Code != http.StatusOK {
		t.Errorf("esperava 200, got %d", rr.Code)
	}
	if textSent == "" {
		t.Error("esperava mensagem de texto para mês vazio")
	}
}

func TestHandleExportCommand_ExporterError_Returns500(t *testing.T) {
	exporter := &mockCSVExporter{
		executeFn: func(_ context.Context, _ time.Time) ([]byte, string, error) {
			return nil, "", errors.New("db error")
		},
	}

	month := time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC)
	h := newHandler(exportAnalyzer(month), &mockMessenger{}, exporter)
	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-EXP-3", false,
		evolutionMessage{Conversation: "exportar março"}, "")
	rr := doRequest(h, body)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("esperava 500, got %d", rr.Code)
	}
}

func TestHandleExportCommand_UsesMonthFromAnalyzer(t *testing.T) {
	var receivedMonth time.Time
	exporter := &mockCSVExporter{
		executeFn: func(_ context.Context, m time.Time) ([]byte, string, error) {
			receivedMonth = m
			return []byte("csv"), "file.csv", nil
		},
	}

	want := time.Date(2024, time.April, 1, 0, 0, 0, 0, time.UTC)
	h := newHandler(exportAnalyzer(want), &mockMessenger{}, exporter)
	body := buildPayload("inst", "5511888888888@s.whatsapp.net", "MSG-EXP-4", false,
		evolutionMessage{Conversation: "exportar abril 2024"}, "")
	doRequest(h, body)

	if !receivedMonth.Equal(want) {
		t.Errorf("esperava mês %v, got %v", want, receivedMonth)
	}
}
