package chat

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

type mockAnalyzer struct {
	textFn   func(usecase.TextInput) (*usecase.ExpenseOutput, error)
	imageFn  func(usecase.ImageInput) (*usecase.ExpenseOutput, error)
	statFn   func() (*usecase.StatementOutput, error)
	saved    []usecase.PendingTransaction
	saveErr  error
	textSeen string
}

func (m *mockAnalyzer) ExecuteText(_ context.Context, in usecase.TextInput) (*usecase.ExpenseOutput, error) {
	m.textSeen = in.Text
	if m.textFn != nil {
		return m.textFn(in)
	}
	return &usecase.ExpenseOutput{Amount: 50, Description: "Almoço", Category: "FOOD", Payment: "PIX"}, nil
}

func (m *mockAnalyzer) ExecuteImage(_ context.Context, in usecase.ImageInput) (*usecase.ExpenseOutput, error) {
	return m.imageFn(in)
}

func (m *mockAnalyzer) ExecuteDocument(context.Context, usecase.DocumentInput) (*usecase.StatementOutput, error) {
	if m.statFn != nil {
		return m.statFn()
	}
	return &usecase.StatementOutput{}, nil
}

func (m *mockAnalyzer) SavePendingTransaction(_ context.Context, tx usecase.PendingTransaction) error {
	m.saved = append(m.saved, tx)
	return m.saveErr
}

type mockMessenger struct {
	texts []string
	docs  []string
	to    string
}

func (m *mockMessenger) SendText(_ context.Context, to, text string) (string, error) {
	m.to = to
	m.texts = append(m.texts, text)
	return "", nil
}

func (m *mockMessenger) SendDocument(_ context.Context, to, filename string, _ []byte, _ string) (string, error) {
	m.to = to
	m.docs = append(m.docs, filename)
	return "", nil
}

type mockExporter struct {
	data []byte
	err  error
	got  time.Time
}

func (m *mockExporter) Execute(_ context.Context, month time.Time) ([]byte, string, *usecase.ExportSummary, error) {
	m.got = month
	return m.data, "gastos.csv", nil, m.err
}

func newTestHandler(a *mockAnalyzer, e *mockExporter) (*Handler, *mockMessenger) {
	msgr := &mockMessenger{}
	return NewHandler(a, e, msgr, "owner-1", slog.New(slog.NewTextHandler(io.Discard, nil))), msgr
}

func loadOf(data []byte, err error) func(context.Context) ([]byte, error) {
	return func(context.Context) ([]byte, error) { return data, err }
}

func TestHandle_Text_RepliesToOwner(t *testing.T) {
	a := &mockAnalyzer{}
	h, msgr := newTestHandler(a, &mockExporter{})

	out, err := h.Handle(context.Background(), Message{Text: "gastei 50 no almoço"})
	if err != nil || out == nil {
		t.Fatalf("esperava output, got %v, %v", out, err)
	}
	if a.textSeen != "gastei 50 no almoço" {
		t.Errorf("texto não repassado: %q", a.textSeen)
	}
	if msgr.to != "owner-1" || len(msgr.texts) != 1 || !strings.Contains(msgr.texts[0], "Despesa registrada") {
		t.Errorf("resposta inesperada: to=%q texts=%v", msgr.to, msgr.texts)
	}
}

func TestHandle_EmptyMessage_NotifiesAndReturnsError(t *testing.T) {
	h, msgr := newTestHandler(&mockAnalyzer{}, &mockExporter{})

	_, err := h.Handle(context.Background(), Message{})
	if !errors.Is(err, ErrUnsupportedMessage) {
		t.Fatalf("esperava ErrUnsupportedMessage, got %v", err)
	}
	if len(msgr.texts) != 1 || !strings.HasPrefix(msgr.texts[0], "Não consegui registrar") {
		t.Errorf("usuário deveria ser avisado: %v", msgr.texts)
	}
}

func TestHandle_Image(t *testing.T) {
	var got usecase.ImageInput
	a := &mockAnalyzer{imageFn: func(in usecase.ImageInput) (*usecase.ExpenseOutput, error) {
		got = in
		return &usecase.ExpenseOutput{Amount: 10}, nil
	}}
	h, _ := newTestHandler(a, &mockExporter{})

	_, err := h.Handle(context.Background(), Message{Image: &Attachment{MimeType: "image/jpeg", Caption: "nota", Load: loadOf([]byte{1, 2}, nil)}})
	if err != nil {
		t.Fatal(err)
	}
	if string(got.ImageData) != "\x01\x02" || got.MimeType != "image/jpeg" || got.Caption != "nota" {
		t.Errorf("input incorreto: %+v", got)
	}
}

func TestHandle_Image_LoadError(t *testing.T) {
	h, _ := newTestHandler(&mockAnalyzer{}, &mockExporter{})

	_, err := h.Handle(context.Background(), Message{Image: &Attachment{Load: loadOf(nil, errors.New("falhou"))}})
	if !errors.Is(err, ErrInvalidImage) {
		t.Errorf("esperava ErrInvalidImage, got %v", err)
	}
}

func exportAnalyzer(month time.Time) *mockAnalyzer {
	return &mockAnalyzer{textFn: func(usecase.TextInput) (*usecase.ExpenseOutput, error) {
		return &usecase.ExpenseOutput{Type: "EXPORT_CSV", ExportMonthTime: month}, nil
	}}
}

func TestHandle_Export_SendsDocument(t *testing.T) {
	month := time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC)
	exp := &mockExporter{data: []byte("csv")}
	h, msgr := newTestHandler(exportAnalyzer(month), exp)

	out, err := h.Handle(context.Background(), Message{Text: "exportar março 2025"})
	if err != nil || out != nil {
		t.Fatalf("esperava (nil, nil), got %v, %v", out, err)
	}
	if len(msgr.docs) != 1 || msgr.docs[0] != "gastos.csv" {
		t.Errorf("documento não enviado: %v", msgr.docs)
	}
	if !exp.got.Equal(month) {
		t.Errorf("mês incorreto: %v", exp.got)
	}
}

func TestHandle_Export_EmptyMonth_SendsText(t *testing.T) {
	month := time.Date(2020, time.February, 1, 0, 0, 0, 0, time.UTC)
	h, msgr := newTestHandler(exportAnalyzer(month), &mockExporter{})

	if _, err := h.Handle(context.Background(), Message{Text: "exportar"}); err != nil {
		t.Fatal(err)
	}
	if len(msgr.docs) != 0 || len(msgr.texts) != 1 || !strings.Contains(msgr.texts[0], "02/2020") {
		t.Errorf("esperava só aviso de mês vazio: docs=%v texts=%v", msgr.docs, msgr.texts)
	}
}

func TestHandle_Export_ExporterError(t *testing.T) {
	h, msgr := newTestHandler(exportAnalyzer(time.Now()), &mockExporter{err: errors.New("db error")})

	if _, err := h.Handle(context.Background(), Message{Text: "exportar"}); err == nil {
		t.Fatal("esperava erro")
	}
	if len(msgr.texts) != 1 || !strings.Contains(msgr.texts[0], "gerar a planilha") {
		t.Errorf("usuário deveria ser avisado: %v", msgr.texts)
	}
}

func pendingStatement(n int) func() (*usecase.StatementOutput, error) {
	return func() (*usecase.StatementOutput, error) {
		items := make([]usecase.PendingTransaction, n)
		for i := range items {
			items[i] = usecase.PendingTransaction{Description: "Tx", Amount: float64(i + 1), Date: time.Now()}
		}
		return &usecase.StatementOutput{Pending: items}, nil
	}
}

func TestHandle_Document_PendingConfirmationFlow(t *testing.T) {
	a := &mockAnalyzer{statFn: pendingStatement(2)}
	h, msgr := newTestHandler(a, &mockExporter{})
	ctx := context.Background()

	if _, err := h.Handle(ctx, Message{Document: &Attachment{Load: loadOf([]byte("pdf"), nil)}}); err != nil {
		t.Fatal(err)
	}
	if last := msgr.texts[len(msgr.texts)-1]; !strings.Contains(last, "Transação 1/2") {
		t.Fatalf("esperava 1ª pergunta, got %q", last)
	}

	h.Handle(ctx, Message{Text: "sim"})
	if last := msgr.texts[len(msgr.texts)-1]; !strings.Contains(last, "Transação 2/2") {
		t.Fatalf("esperava 2ª pergunta, got %q", last)
	}

	h.Handle(ctx, Message{Text: "não"})
	if last := msgr.texts[len(msgr.texts)-1]; !strings.Contains(last, "Importação concluída") {
		t.Fatalf("esperava conclusão, got %q", last)
	}
	if len(a.saved) != 1 || a.saved[0].Amount != 1 {
		t.Errorf("só a 1ª transação deveria ser salva: %+v", a.saved)
	}
}

func TestHandle_SimWithoutPending_GoesToAnalyzer(t *testing.T) {
	a := &mockAnalyzer{}
	h, _ := newTestHandler(a, &mockExporter{})

	h.Handle(context.Background(), Message{Text: "sim"})
	if a.textSeen != "sim" {
		t.Error("sem importação pendente, 'sim' deve seguir o fluxo normal")
	}
}

func TestHandle_Document_LoadError(t *testing.T) {
	h, msgr := newTestHandler(&mockAnalyzer{}, &mockExporter{})

	out, err := h.Handle(context.Background(), Message{Document: &Attachment{Load: loadOf(nil, errors.New("falhou"))}})
	if out != nil || err != nil {
		t.Fatalf("falha de documento é tratada ao usuário, got %v, %v", out, err)
	}
	if len(msgr.texts) != 1 || !strings.Contains(msgr.texts[0], "Não consegui ler o documento") {
		t.Errorf("mensagem inesperada: %v", msgr.texts)
	}
}
