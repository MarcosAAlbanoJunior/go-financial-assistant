package usecase

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

type CSVExporter interface {
	Execute(ctx context.Context, month time.Time) (data []byte, filename string, err error)
}

type ExportCSV struct {
	repo ports.PurchaseRepository
}

func NewExportCSV(repo ports.PurchaseRepository) *ExportCSV {
	return &ExportCSV{repo: repo}
}

func (uc *ExportCSV) Execute(ctx context.Context, month time.Time) ([]byte, string, error) {
	details, err := uc.repo.FindPaymentDetailsByMonth(ctx, month)
	if err != nil {
		return nil, "", fmt.Errorf("erro ao buscar despesas: %w", err)
	}

	if len(details) == 0 {
		return nil, "", nil
	}

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")

	w := csv.NewWriter(&buf)

	if err := w.Write([]string{"Data", "Descrição", "Categoria", "Forma de Pagamento", "Tipo", "Parcela", "Valor (R$)"}); err != nil {
		return nil, "", fmt.Errorf("erro ao escrever cabeçalho: %w", err)
	}

	var total float64
	for _, d := range details {
		total += d.Amount
		row := buildCSVRow(d)
		if err := w.Write(row); err != nil {
			return nil, "", fmt.Errorf("erro ao escrever linha: %w", err)
		}
	}

	totalRow := []string{"", "TOTAL", "", "", "", "", fmt.Sprintf("%.2f", total)}
	if err := w.Write(totalRow); err != nil {
		return nil, "", fmt.Errorf("erro ao escrever linha de total: %w", err)
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", fmt.Errorf("erro ao finalizar CSV: %w", err)
	}

	filename := fmt.Sprintf("despesas_%s_%d.csv",
		strings.ToLower(ptMonths[month.Month()-1]),
		month.Year(),
	)

	return buf.Bytes(), filename, nil
}

func buildCSVRow(d ports.PaymentDetail) []string {
	date := resolvePaymentDate(d)

	desc := ""
	if d.Description != nil {
		desc = *d.Description
	}

	installment := "-"
	if d.InstallmentNumber != nil {
		installment = fmt.Sprintf("%d", *d.InstallmentNumber)
	}

	return []string{
		date.Format("02/01/2006"),
		desc,
		domain.Category(d.Category).Label(),
		domain.PaymentMethod(d.PaymentMethod).Label(),
		purchaseTypeLabel(d.PurchaseType),
		installment,
		fmt.Sprintf("%.2f", d.Amount),
	}
}

func resolvePaymentDate(d ports.PaymentDetail) time.Time {
	if d.DueDate != nil {
		return *d.DueDate
	}
	if d.ReferenceMonth != nil {
		return *d.ReferenceMonth
	}
	return d.CreatedAt
}

func purchaseTypeLabel(t string) string {
	switch t {
	case "INSTALLMENT":
		return "Parcelado"
	case "RECURRING":
		return "Recorrente"
	default:
		return "Único"
	}
}
