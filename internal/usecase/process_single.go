package usecase

import (
	"context"
	"fmt"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func (uc *AnalyzeExpense) processAnalysis(
	ctx context.Context,
	analysis *ports.ExpenseAnalysis,
	paymentMethod string,
	rawInput string,
) (*ExpenseOutput, error) {
	if analysis.Amount == nil {
		return nil, fmt.Errorf(
			"não foi possível identificar o valor da despesa (confiança: %.0f%%)",
			analysis.Confidence*100,
		)
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

	expense, err := domain.NewExpense(
		*analysis.Amount,
		description,
		category,
		payment,
		rawInput,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("despesa inválida: %w", err)
	}

	if err := uc.repo.Save(ctx, expense); err != nil {
		return nil, fmt.Errorf("erro ao salvar despesa: %w", err)
	}

	return &ExpenseOutput{
		ID:          expense.ID.String(),
		Amount:      expense.Amount,
		Description: expense.Description,
		Category:    string(expense.Category),
		Payment:     string(expense.Payment),
		Confidence:  analysis.Confidence,
		Type:        string(ports.ExpenseTypeSingle),
	}, nil
}
