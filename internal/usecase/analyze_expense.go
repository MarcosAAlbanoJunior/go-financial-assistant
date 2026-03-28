package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

type ExpenseAnalyzer interface {
	ExecuteText(ctx context.Context, input TextInput) (*ExpenseOutput, error)
	ExecuteImage(ctx context.Context, input ImageInput) (*ExpenseOutput, error)
}

type AnalyzeExpense struct {
	repo     ports.PurchaseRepository
	analyzer ports.AIAnalyzer
	logger   *slog.Logger
}

func NewAnalyzeExpense(repo ports.PurchaseRepository, analyzer ports.AIAnalyzer, logger *slog.Logger) *AnalyzeExpense {
	return &AnalyzeExpense{repo: repo, analyzer: analyzer, logger: logger}
}

type TextInput struct {
	Text string
}

type ImageInput struct {
	ImageData []byte
	MimeType  string
	Caption   string
}

type CategorySummary struct {
	Category string  `json:"category"`
	Total    float64 `json:"total"`
}

type ExpenseOutput struct {
	ID          string  `json:"id"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Payment     string  `json:"payment"`
	Confidence  float64 `json:"confidence"`
	Type        string  `json:"type"`

	TotalInstallments int     `json:"total_installments,omitempty"`
	InstallmentAmount float64 `json:"installment_amount,omitempty"`

	DayOfMonth int `json:"day_of_month,omitempty"`

	Cancelled            bool   `json:"cancelled,omitempty"`
	CancelledDescription string `json:"cancelled_description,omitempty"`

	QueryMonth      string            `json:"query_month,omitempty"`
	QueryTotal      float64           `json:"query_total,omitempty"`
	QueryCategories []CategorySummary `json:"query_categories,omitempty"`
	QueryEmpty      bool              `json:"query_empty,omitempty"`

	ExportMonthTime time.Time `json:"-"`
}

func (uc *AnalyzeExpense) ExecuteText(ctx context.Context, input TextInput) (*ExpenseOutput, error) {
	analysis, err := uc.analyzer.AnalyzeText(ctx, input.Text)
	if err != nil {
		return nil, fmt.Errorf("erro ao analisar texto: %w", err)
	}

	payment := resolvePaymentMethod(analysis.PaymentMethod, inferPaymentMethod(input.Text))

	switch analysis.Type {
	case ports.ExpenseTypeQuery:
		return uc.processQuery(ctx, analysis)
	case ports.ExpenseTypeExportCSV:
		return processExportCSV(analysis)
	case ports.ExpenseTypeInstallment:
		return uc.processInstallment(ctx, analysis, payment, input.Text)
	case ports.ExpenseTypeRecurring:
		return uc.processRecurring(ctx, analysis, payment, input.Text)
	case ports.ExpenseTypeCancelRecurring:
		return uc.processCancel(ctx, analysis)
	default:
		return uc.processAnalysis(ctx, analysis, payment, input.Text)
	}
}

func (uc *AnalyzeExpense) ExecuteImage(ctx context.Context, input ImageInput) (*ExpenseOutput, error) {
	analysis, err := uc.analyzer.AnalyzeImage(ctx, input.ImageData, input.MimeType)
	if err != nil {
		return nil, fmt.Errorf("erro ao analisar imagem: %w", err)
	}

	payment := resolvePaymentMethod(analysis.PaymentMethod, inferPaymentMethod(input.Caption))
	rawInput := fmt.Sprintf("[imagem: %s]", input.MimeType)

	switch analysis.Type {
	case ports.ExpenseTypeExportCSV:
		return processExportCSV(analysis)
	case ports.ExpenseTypeInstallment:
		return uc.processInstallment(ctx, analysis, payment, rawInput)
	case ports.ExpenseTypeRecurring:
		return uc.processRecurring(ctx, analysis, payment, rawInput)
	case ports.ExpenseTypeCancelRecurring:
		return uc.processCancel(ctx, analysis)
	default:
		return uc.processAnalysis(ctx, analysis, payment, rawInput)
	}
}

func (uc *AnalyzeExpense) GenerateRecurringExpenses(ctx context.Context) error {
	actives, err := uc.repo.FindActiveRecurring(ctx)
	if err != nil {
		return fmt.Errorf("erro ao buscar despesas recorrentes: %w", err)
	}

	now := time.Now().UTC()
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	for i := range actives {
		p := &actives[i]

		if p.DayOfMonth != nil {
			target := lastValidDay(now.Year(), now.Month(), *p.DayOfMonth)
			if now.Day() != target {
				continue
			}
		}

		has, err := uc.repo.HasPaymentForMonth(ctx, p.ID, firstOfMonth)
		if err != nil {
			uc.logger.Error("erro ao verificar pagamento mensal", "purchase_id", p.ID, "error", err)
			continue
		}
		if has {
			continue
		}
		payment := domain.NewPayment(p.ID, p.TotalAmount, domain.PaymentStatusPaid)
		payment.ReferenceMonth = &firstOfMonth
		if err := uc.repo.SavePayment(ctx, payment); err != nil {
			uc.logger.Error("erro ao salvar pagamento recorrente", "purchase_id", p.ID, "error", err)
		}
	}

	return nil
}

func processExportCSV(analysis *ports.ExpenseAnalysis) (*ExpenseOutput, error) {
	now := time.Now().UTC()
	month := now.Month()
	year := now.Year()

	if analysis.ExportInfo != nil {
		if analysis.ExportInfo.Month >= 1 && analysis.ExportInfo.Month <= 12 {
			month = time.Month(analysis.ExportInfo.Month)
		}
		if analysis.ExportInfo.Year >= 2000 {
			year = analysis.ExportInfo.Year
		}
	}

	return &ExpenseOutput{
		Type:            string(ports.ExpenseTypeExportCSV),
		ExportMonthTime: time.Date(year, month, 1, 0, 0, 0, 0, time.UTC),
	}, nil
}

// lastValidDay retorna o menor valor entre day e o último dia do mês.
// Garante que dia 31 em fevereiro, por exemplo, vira dia 28/29.
func lastValidDay(year int, month time.Month, day int) int {
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		return lastDay
	}
	return day
}
