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
}

func (uc *AnalyzeExpense) ExecuteText(ctx context.Context, input TextInput) (*ExpenseOutput, error) {
	analysis, err := uc.analyzer.AnalyzeText(ctx, input.Text)
	if err != nil {
		return nil, fmt.Errorf("erro ao analisar texto: %w", err)
	}

	payment := resolvePaymentMethod(analysis.PaymentMethod, inferPaymentMethod(input.Text))

	switch analysis.Type {
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
