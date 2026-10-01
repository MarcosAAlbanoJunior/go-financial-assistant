package usecase

import (
	"context"
	"fmt"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func (uc *AnalyzeExpense) processTransfer(
	ctx context.Context,
	analysis *ports.ExpenseAnalysis,
	paymentMethod string,
	rawInput string,
) (*ExpenseOutput, error) {
	if analysis.Amount == nil {
		return nil, fmt.Errorf(
			"não foi possível identificar o valor da transferência (confiança: %.0f%%)",
			analysis.Confidence*100,
		)
	}

	payment, err := parsePaymentMethod(paymentMethod)
	if err != nil {
		return nil, err
	}

	transfer, err := domain.NewTransfer(
		*analysis.Amount,
		extractDescription(analysis.Description),
		payment,
		domain.PurchaseTypeSingle,
		rawInput,
		domain.ParseTransferDirection(analysis.TransferDirection),
	)
	if err != nil {
		return nil, fmt.Errorf("transferência inválida: %w", err)
	}

	pmt := domain.NewPayment(transfer.ID, *analysis.Amount, domain.PaymentStatusPaid)

	if err := uc.repo.Save(ctx, transfer, []domain.Payment{*pmt}); err != nil {
		return nil, fmt.Errorf("erro ao salvar transferência: %w", err)
	}

	return &ExpenseOutput{
		ID:          transfer.ID.String(),
		Amount:      transfer.TotalAmount,
		Description: descriptionOrFallback(transfer.Description, rawInput),
		Payment:     transfer.PaymentMethod.Label(),
		Confidence:  analysis.Confidence,
		Type:        string(ports.ExpenseTypeTransfer),
	}, nil
}

