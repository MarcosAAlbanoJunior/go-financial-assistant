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

	purchase, err := domain.NewPurchase(
		*analysis.Amount,
		extractDescription(analysis.Description),
		parseCategory(analysis.Category),
		payment,
		domain.PurchaseTypeSingle,
		rawInput,
	)
	if err != nil {
		return nil, fmt.Errorf("despesa inválida: %w", err)
	}

	status := domain.PaymentStatusPending
	if payment != domain.PaymentMethodCreditCard {
		status = domain.PaymentStatusPaid
	}

	pmt := domain.NewPayment(purchase.ID, *analysis.Amount, status)

	if err := uc.repo.Save(ctx, purchase, []domain.Payment{*pmt}); err != nil {
		return nil, fmt.Errorf("erro ao salvar despesa: %w", err)
	}

	return &ExpenseOutput{
		ID:          purchase.ID.String(),
		Amount:      purchase.TotalAmount,
		Description: descriptionOrFallback(purchase.Description, rawInput),
		Category:    string(purchase.Category),
		Payment:     string(purchase.PaymentMethod),
		Confidence:  analysis.Confidence,
		Type:        string(ports.ExpenseTypeSingle),
	}, nil
}
