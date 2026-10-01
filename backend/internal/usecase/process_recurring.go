package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func (uc *AnalyzeExpense) processRecurring(
	ctx context.Context,
	analysis *ports.ExpenseAnalysis,
	paymentMethod string,
	rawInput string,
) (*ExpenseOutput, error) {
	if analysis.Amount == nil {
		return nil, fmt.Errorf("valor não identificado para despesa recorrente")
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

	purchase, err := domain.NewPurchase(
		*analysis.Amount,
		extractDescription(analysis.Description),
		parseCategory(analysis.Category),
		payment,
		domain.PurchaseTypeRecurring,
		rawInput,
	)
	if err != nil {
		return nil, fmt.Errorf("despesa recorrente inválida: %w", err)
	}
	purchase.DayOfMonth = &dayOfMonth

	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	firstPayment := domain.NewPayment(purchase.ID, *analysis.Amount, domain.PaymentStatusPaid)
	firstPayment.ReferenceMonth = &firstOfMonth

	if err := uc.repo.Save(ctx, purchase, []domain.Payment{*firstPayment}); err != nil {
		return nil, fmt.Errorf("erro ao salvar despesa recorrente: %w", err)
	}

	return &ExpenseOutput{
		ID:          purchase.ID.String(),
		Amount:      purchase.TotalAmount,
		Description: descriptionOrFallback(purchase.Description, rawInput),
		Category:    purchase.Category.Label(),
		Payment:     purchase.PaymentMethod.Label(),
		Confidence:  analysis.Confidence,
		Type:        string(ports.ExpenseTypeRecurring),
		DayOfMonth:  dayOfMonth,
	}, nil
}

func (uc *AnalyzeExpense) processCancel(ctx context.Context, analysis *ports.ExpenseAnalysis) (*ExpenseOutput, error) {
	searchDesc := ""
	if analysis.CancelInfo != nil && analysis.CancelInfo.Description != "" {
		searchDesc = analysis.CancelInfo.Description
	} else if analysis.Description != nil {
		searchDesc = *analysis.Description
	}

	if searchDesc == "" {
		return nil, fmt.Errorf("não foi possível identificar qual despesa recorrente cancelar")
	}

	matches, err := uc.repo.FindByDescription(ctx, searchDesc)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar despesa recorrente: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("nenhuma despesa recorrente ativa encontrada com descrição '%s'", searchDesc)
	}
	if len(matches) > 1 {
		uc.logger.Warn("múltiplas despesas recorrentes encontradas, cancelando a primeira", "description", searchDesc, "count", len(matches))
	}

	purchase := matches[0]
	purchase.Cancel("cancelado pelo usuário")

	if err := uc.repo.Update(ctx, &purchase); err != nil {
		return nil, fmt.Errorf("erro ao cancelar despesa recorrente: %w", err)
	}

	cancelledDesc := descriptionOrFallback(purchase.Description, purchase.RawInput)
	return &ExpenseOutput{
		ID:                   purchase.ID.String(),
		Amount:               purchase.TotalAmount,
		Description:          cancelledDesc,
		Category:             purchase.Category.Label(),
		Payment:              purchase.PaymentMethod.Label(),
		Confidence:           analysis.Confidence,
		Type:                 string(ports.ExpenseTypeCancelRecurring),
		Cancelled:            true,
		CancelledDescription: cancelledDesc,
	}, nil
}
