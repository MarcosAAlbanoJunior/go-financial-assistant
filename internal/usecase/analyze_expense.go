package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

type ExpenseAnalyzer interface {
	ExecuteText(ctx context.Context, input TextInput) (*ExpenseOutput, error)
	ExecuteImage(ctx context.Context, input ImageInput) (*ExpenseOutput, error)
}

type AnalyzeExpense struct {
	repo          ports.ExpenseRepository
	installRepo   ports.InstallmentRepository
	recurringRepo ports.RecurringExpenseRepository
	analyzer      ports.AIAnalyzer
	logger        *slog.Logger
}

func NewAnalyzeExpense(
	repo ports.ExpenseRepository,
	installRepo ports.InstallmentRepository,
	recurringRepo ports.RecurringExpenseRepository,
	analyzer ports.AIAnalyzer,
	logger *slog.Logger,
) *AnalyzeExpense {
	return &AnalyzeExpense{
		repo:          repo,
		installRepo:   installRepo,
		recurringRepo: recurringRepo,
		analyzer:      analyzer,
		logger:        logger,
	}
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
	actives, err := uc.recurringRepo.FindActive(ctx)
	if err != nil {
		return fmt.Errorf("erro ao buscar despesas recorrentes: %w", err)
	}

	now := time.Now().UTC()
	year, month, _ := now.Date()

	for i := range actives {
		r := &actives[i]
		if !r.ShouldGenerateForMonth(year, month) {
			continue
		}
		expense, err := r.GenerateExpense()
		if err != nil {
			uc.logger.Error("erro ao gerar despesa recorrente", "recurring_id", r.ID, "error", err)
			continue
		}
		if err := uc.repo.Save(ctx, expense); err != nil {
			uc.logger.Error("erro ao salvar despesa recorrente gerada", "recurring_id", r.ID, "error", err)
			continue
		}
		r.LastGeneratedDate = &now
		if err := uc.recurringRepo.Update(ctx, r); err != nil {
			uc.logger.Error("erro ao atualizar data de geração da despesa recorrente", "recurring_id", r.ID, "error", err)
		}
	}

	return nil
}
