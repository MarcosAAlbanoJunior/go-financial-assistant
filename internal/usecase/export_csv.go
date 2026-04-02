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

type ExportSummary struct {
	TotalExpenses float64
	TotalIncome   float64
	Balance       float64
}

type CSVExporter interface {
	Execute(ctx context.Context, month time.Time) (data []byte, filename string, summary *ExportSummary, err error)
}

type ExportCSV struct {
	repo ports.PurchaseRepository
}

func NewExportCSV(repo ports.PurchaseRepository) *ExportCSV {
	return &ExportCSV{repo: repo}
}

func (uc *ExportCSV) Execute(ctx context.Context, month time.Time) ([]byte, string, *ExportSummary, error) {
	details, err := uc.repo.FindPaymentDetailsByMonth(ctx, month)
	if err != nil {
		return nil, "", nil, fmt.Errorf("erro ao buscar despesas: %w", err)
	}

	if len(details) == 0 {
		return nil, "", nil, nil
	}

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")

	w := csv.NewWriter(&buf)

	if err := w.Write([]string{"Data", "Descrição", "Categoria", "Forma de Pagamento", "Tipo", "Parcela", "Valor (R$)"}); err != nil {
		return nil, "", nil, fmt.Errorf("erro ao escrever cabeçalho: %w", err)
	}

	var totalExpenses, totalIncome float64
	for _, d := range details {
		row := buildCSVRow(d)
		if err := w.Write(row); err != nil {
			return nil, "", nil, fmt.Errorf("erro ao escrever linha: %w", err)
		}
		if d.PurchaseKind == "INCOME" {
			totalIncome += d.Amount
		} else {
			totalExpenses += d.Amount
		}
	}

	if err := w.Write([]string{"", "TOTAL DESPESAS", "", "", "", "", fmt.Sprintf("%.2f", totalExpenses)}); err != nil {
		return nil, "", nil, fmt.Errorf("erro ao escrever total despesas: %w", err)
	}
	if totalIncome > 0 {
		if err := w.Write([]string{"", "TOTAL ENTRADAS", "", "", "", "", fmt.Sprintf("%.2f", totalIncome)}); err != nil {
			return nil, "", nil, fmt.Errorf("erro ao escrever total entradas: %w", err)
		}
		balance := totalIncome - totalExpenses
		if err := w.Write([]string{"", "SALDO", "", "", "", "", fmt.Sprintf("%.2f", balance)}); err != nil {
			return nil, "", nil, fmt.Errorf("erro ao escrever saldo: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", nil, fmt.Errorf("erro ao finalizar CSV: %w", err)
	}

	filename := fmt.Sprintf("despesas_%s_%d.csv",
		strings.ToLower(ptMonths[month.Month()-1]),
		month.Year(),
	)

	summary := &ExportSummary{
		TotalExpenses: totalExpenses,
		TotalIncome:   totalIncome,
		Balance:       totalIncome - totalExpenses,
	}

	return buf.Bytes(), filename, summary, nil
}

func BuildExportCaption(month time.Time, summary *ExportSummary) string {
	base := fmt.Sprintf("📊 Planilha de %s %d\n", ptMonths[month.Month()-1], month.Year())
	if summary == nil {
		return base
	}
	caption := base
	caption += fmt.Sprintf("💸 Despesas: R$ %.2f\n", summary.TotalExpenses)
	if summary.TotalIncome > 0 {
		caption += fmt.Sprintf("💰 Entradas: R$ %.2f\n", summary.TotalIncome)
		caption += fmt.Sprintf("📈 Saldo: R$ %.2f", summary.Balance)
	}
	return caption
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
		purchaseKindTypeLabel(d.PurchaseKind, d.PurchaseType),
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

func purchaseKindTypeLabel(kind, t string) string {
	if kind == "INCOME" {
		if t == "RECURRING" {
			return "Entrada Recorrente"
		}
		return "Entrada"
	}
	switch t {
	case "INSTALLMENT":
		return "Parcelado"
	case "RECURRING":
		return "Recorrente"
	default:
		return "Único"
	}
}
