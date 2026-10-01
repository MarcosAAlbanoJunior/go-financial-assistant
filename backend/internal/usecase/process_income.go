package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func (uc *AnalyzeExpense) processIncome(
	ctx context.Context,
	analysis *ports.ExpenseAnalysis,
	paymentMethod string,
	rawInput string,
) (*ExpenseOutput, error) {
	if analysis.Amount == nil {
		return nil, fmt.Errorf("não foi possível identificar o valor da entrada (confiança: %.0f%%)", analysis.Confidence*100)
	}

	payment, err := parsePaymentMethod(paymentMethod)
	if err != nil {
		return nil, err
	}

	income, err := domain.NewIncome(
		*analysis.Amount,
		extractDescription(analysis.Description),
		parseCategory(analysis.Category),
		payment,
		domain.PurchaseTypeSingle,
		rawInput,
	)
	if err != nil {
		return nil, fmt.Errorf("entrada inválida: %w", err)
	}

	pmt := domain.NewPayment(income.ID, *analysis.Amount, domain.PaymentStatusPaid)

	if err := uc.repo.Save(ctx, income, []domain.Payment{*pmt}); err != nil {
		return nil, fmt.Errorf("erro ao salvar entrada: %w", err)
	}

	return &ExpenseOutput{
		ID:          income.ID.String(),
		Amount:      income.TotalAmount,
		Description: descriptionOrFallback(income.Description, rawInput),
		Category:    income.Category.Label(),
		Payment:     income.PaymentMethod.Label(),
		Confidence:  analysis.Confidence,
		Type:        string(ports.ExpenseTypeIncome),
	}, nil
}

func (uc *AnalyzeExpense) processIncomeRecurring(
	ctx context.Context,
	analysis *ports.ExpenseAnalysis,
	paymentMethod string,
	rawInput string,
) (*ExpenseOutput, error) {
	if analysis.Amount == nil {
		return nil, fmt.Errorf("valor não identificado para entrada recorrente")
	}

	payment, err := parsePaymentMethod(paymentMethod)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	dayOfMonth := now.Day()
	if analysis.RecurringInfo != nil && analysis.RecurringInfo.DayOfMonth >= 1 && analysis.RecurringInfo.DayOfMonth <= 31 {
		dayOfMonth = analysis.RecurringInfo.DayOfMonth
	}

	income, err := domain.NewIncome(
		*analysis.Amount,
		extractDescription(analysis.Description),
		parseCategory(analysis.Category),
		payment,
		domain.PurchaseTypeRecurring,
		rawInput,
	)
	if err != nil {
		return nil, fmt.Errorf("entrada recorrente inválida: %w", err)
	}
	income.DayOfMonth = &dayOfMonth

	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	firstPayment := domain.NewPayment(income.ID, *analysis.Amount, domain.PaymentStatusPaid)
	firstPayment.ReferenceMonth = &firstOfMonth

	if err := uc.repo.Save(ctx, income, []domain.Payment{*firstPayment}); err != nil {
		return nil, fmt.Errorf("erro ao salvar entrada recorrente: %w", err)
	}

	return &ExpenseOutput{
		ID:          income.ID.String(),
		Amount:      income.TotalAmount,
		Description: descriptionOrFallback(income.Description, rawInput),
		Category:    income.Category.Label(),
		Payment:     income.PaymentMethod.Label(),
		Confidence:  analysis.Confidence,
		Type:        string(ports.ExpenseTypeIncomeRecurring),
		DayOfMonth:  dayOfMonth,
	}, nil
}
