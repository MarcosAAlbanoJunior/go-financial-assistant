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

	category := parseCategory(analysis.Category)
	description := rawInput
	if analysis.Description != nil && *analysis.Description != "" {
		description = *analysis.Description
	}

	dayOfMonth := 1
	if analysis.RecurringInfo != nil && analysis.RecurringInfo.DayOfMonth >= 1 && analysis.RecurringInfo.DayOfMonth <= 28 {
		dayOfMonth = analysis.RecurringInfo.DayOfMonth
	}

	recurring, err := domain.NewRecurringExpense(description, *analysis.Amount, category, payment, dayOfMonth, rawInput)
	if err != nil {
		return nil, fmt.Errorf("despesa recorrente inválida: %w", err)
	}

	if err := uc.recurringRepo.Save(ctx, recurring); err != nil {
		return nil, fmt.Errorf("erro ao salvar despesa recorrente: %w", err)
	}

	expense, err := recurring.GenerateExpense()
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar primeira despesa recorrente: %w", err)
	}

	if err := uc.repo.Save(ctx, expense); err != nil {
		return nil, fmt.Errorf("erro ao salvar primeira despesa recorrente: %w", err)
	}

	now := time.Now().UTC()
	recurring.LastGeneratedDate = &now
	if err := uc.recurringRepo.Update(ctx, recurring); err != nil {
		uc.logger.Error("erro ao atualizar data de geração da despesa recorrente", "recurring_id", recurring.ID, "error", err)
	}

	return &ExpenseOutput{
		ID:          recurring.ID.String(),
		Amount:      recurring.Amount,
		Description: recurring.Description,
		Category:    string(recurring.Category),
		Payment:     string(recurring.Payment),
		Confidence:  analysis.Confidence,
		Type:        string(ports.ExpenseTypeRecurring),
		DayOfMonth:  recurring.DayOfMonth,
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

	matches, err := uc.recurringRepo.FindByDescription(ctx, searchDesc)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar despesa recorrente: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("nenhuma despesa recorrente ativa encontrada com descrição '%s'", searchDesc)
	}
	if len(matches) > 1 {
		uc.logger.Warn("múltiplas despesas recorrentes encontradas, cancelando a primeira", "description", searchDesc, "count", len(matches))
	}

	recurring := matches[0]
	recurring.Cancel("cancelado pelo usuário")

	if err := uc.recurringRepo.Update(ctx, &recurring); err != nil {
		return nil, fmt.Errorf("erro ao cancelar despesa recorrente: %w", err)
	}

	return &ExpenseOutput{
		ID:                   recurring.ID.String(),
		Amount:               recurring.Amount,
		Description:          recurring.Description,
		Category:             string(recurring.Category),
		Payment:              string(recurring.Payment),
		Confidence:           analysis.Confidence,
		Type:                 string(ports.ExpenseTypeCancelRecurring),
		Cancelled:            true,
		CancelledDescription: recurring.Description,
	}, nil
}
