package usecase

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func TestExportCSV_EmptyMonth(t *testing.T) {
	repo := &mockPurchaseRepo{
		findPaymentDetailsByMonthFn: func(_ context.Context, _ time.Time) ([]ports.PaymentDetail, error) {
			return nil, nil
		},
	}

	uc := NewExportCSV(repo)
	data, filename, err := uc.Execute(context.Background(), time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))

	if err != nil {
		t.Fatalf("esperava nil error, got: %v", err)
	}
	if data != nil {
		t.Error("esperava data == nil para mês sem despesas")
	}
	if filename != "" {
		t.Errorf("esperava filename vazio, got: %q", filename)
	}
}

func TestExportCSV_RepoError(t *testing.T) {
	repo := &mockPurchaseRepo{
		findPaymentDetailsByMonthFn: func(_ context.Context, _ time.Time) ([]ports.PaymentDetail, error) {
			return nil, errSentinel
		},
	}

	uc := NewExportCSV(repo)
	_, _, err := uc.Execute(context.Background(), time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))

	if err == nil {
		t.Fatal("esperava erro do repositório")
	}
}

func TestExportCSV_Filename(t *testing.T) {
	repo := repoWithOneDetail(singleDetail())

	uc := NewExportCSV(repo)
	_, filename, err := uc.Execute(context.Background(), time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))

	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if filename != "despesas_março_2025.csv" {
		t.Errorf("filename inesperado: %q", filename)
	}
}

func TestExportCSV_HasBOM(t *testing.T) {
	repo := repoWithOneDetail(singleDetail())

	uc := NewExportCSV(repo)
	data, _, _ := uc.Execute(context.Background(), time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))

	bom := []byte{0xEF, 0xBB, 0xBF}
	if !bytes.HasPrefix(data, bom) {
		t.Error("CSV deve começar com BOM UTF-8")
	}
}

func TestExportCSV_Header(t *testing.T) {
	repo := repoWithOneDetail(singleDetail())

	uc := NewExportCSV(repo)
	data, _, _ := uc.Execute(context.Background(), time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))

	records := parseCSV(t, data)
	if len(records) < 1 {
		t.Fatal("CSV sem linhas")
	}
	header := records[0]
	expected := []string{"Data", "Descrição", "Categoria", "Forma de Pagamento", "Tipo", "Parcela", "Valor (R$)"}
	for i, col := range expected {
		if i >= len(header) || header[i] != col {
			t.Errorf("coluna %d: esperava %q, got %q", i, col, header[i])
		}
	}
}

func TestExportCSV_SingleRow(t *testing.T) {
	due := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
	detail := ports.PaymentDetail{
		Description:   strPtr("Almoço"),
		Category:      "FOOD",
		PaymentMethod: "PIX",
		Amount:        45.50,
		Status:        "PAID",
		PurchaseType:  "SINGLE",
		DueDate:       &due,
	}

	repo := repoWithOneDetail(detail)
	uc := NewExportCSV(repo)
	data, _, _ := uc.Execute(context.Background(), time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))

	records := parseCSV(t, data)
	if len(records) < 2 {
		t.Fatal("CSV sem linha de dados")
	}

	row := records[1]
	if row[0] != "15/03/2025" {
		t.Errorf("data: esperava 15/03/2025, got %q", row[0])
	}
	if row[1] != "Almoço" {
		t.Errorf("descrição: esperava Almoço, got %q", row[1])
	}
	if row[2] != "Alimentação" {
		t.Errorf("categoria: esperava Alimentação, got %q", row[2])
	}
	if row[3] != "Pix" {
		t.Errorf("forma pagamento: esperava Pix, got %q", row[3])
	}
	if row[4] != "Único" {
		t.Errorf("tipo: esperava Único, got %q", row[4])
	}
	if row[5] != "-" {
		t.Errorf("parcela: esperava -, got %q", row[5])
	}
	if row[6] != "45.50" {
		t.Errorf("valor: esperava 45.50, got %q", row[6])
	}
}

func TestExportCSV_InstallmentRow(t *testing.T) {
	due := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)
	installNum := 2
	detail := ports.PaymentDetail{
		Description:       strPtr("Notebook"),
		Category:          "SHOPPING",
		PaymentMethod:     "CREDIT_CARD",
		Amount:            500.00,
		PurchaseType:      "INSTALLMENT",
		InstallmentNumber: &installNum,
		DueDate:           &due,
	}

	repo := repoWithOneDetail(detail)
	uc := NewExportCSV(repo)
	data, _, _ := uc.Execute(context.Background(), time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))

	records := parseCSV(t, data)
	row := records[1]

	if row[4] != "Parcelado" {
		t.Errorf("tipo: esperava Parcelado, got %q", row[4])
	}
	if row[5] != "2" {
		t.Errorf("parcela: esperava 2, got %q", row[5])
	}
}

func TestExportCSV_RecurringRow(t *testing.T) {
	ref := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	detail := ports.PaymentDetail{
		Description:    strPtr("Netflix"),
		Category:       "ENTERTAINMENT",
		PaymentMethod:  "CREDIT_CARD",
		Amount:         55.90,
		PurchaseType:   "RECURRING",
		ReferenceMonth: &ref,
	}

	repo := repoWithOneDetail(detail)
	uc := NewExportCSV(repo)
	data, _, _ := uc.Execute(context.Background(), time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))

	records := parseCSV(t, data)
	row := records[1]

	if row[4] != "Recorrente" {
		t.Errorf("tipo: esperava Recorrente, got %q", row[4])
	}
	if row[0] != "01/03/2025" {
		t.Errorf("data: esperava 01/03/2025, got %q", row[0])
	}
}

func TestExportCSV_NilDescription(t *testing.T) {
	due := time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC)
	detail := ports.PaymentDetail{
		Description:   nil,
		Category:      "OTHER",
		PaymentMethod: "CASH",
		Amount:        10.00,
		PurchaseType:  "SINGLE",
		DueDate:       &due,
	}

	repo := repoWithOneDetail(detail)
	uc := NewExportCSV(repo)
	data, _, _ := uc.Execute(context.Background(), time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))

	records := parseCSV(t, data)
	if records[1][1] != "" {
		t.Errorf("descrição nil deveria gerar string vazia, got %q", records[1][1])
	}
}

// helpers

var errSentinel = errors.New("repo error")

func strPtr(s string) *string { return &s }

func singleDetail() ports.PaymentDetail {
	due := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	return ports.PaymentDetail{
		Description:   strPtr("Teste"),
		Category:      "FOOD",
		PaymentMethod: "PIX",
		Amount:        10.00,
		PurchaseType:  "SINGLE",
		DueDate:       &due,
	}
}

func repoWithOneDetail(d ports.PaymentDetail) *mockPurchaseRepo {
	return &mockPurchaseRepo{
		findPaymentDetailsByMonthFn: func(_ context.Context, _ time.Time) ([]ports.PaymentDetail, error) {
			return []ports.PaymentDetail{d}, nil
		},
	}
}

func parseCSV(t *testing.T, data []byte) [][]string {
	t.Helper()
	// Remove BOM before parsing
	content := strings.TrimPrefix(string(data), "\xEF\xBB\xBF")
	r := csv.NewReader(strings.NewReader(content))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("erro ao parsear CSV: %v", err)
	}
	return records
}
