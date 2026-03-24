package usecase

import (
	"context"
	"fmt"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func (uc *AnalyzeExpense) processInstallment(
	ctx context.Context,
	analysis *ports.ExpenseAnalysis,
	paymentMethod string,
	rawInput string,
) (*ExpenseOutput, error) {
	if analysis.Amount == nil {
		return nil, fmt.Errorf("valor total não identificado para compra parcelada")
	}
	if analysis.Installments == nil || analysis.Installments.Total <= 0 {
		return uc.processAnalysis(ctx, analysis, paymentMethod, rawInput)
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

	totalAmount := *analysis.Amount
	installmentAmount := analysis.Installments.AmountPerInstallment
	if installmentAmount <= 0 {
		installmentAmount = totalAmount / float64(analysis.Installments.Total)
	}

	purchase, installments, err := domain.NewInstallmentPurchase(
		description,
		totalAmount,
		installmentAmount,
		analysis.Installments.Total,
		category,
		payment,
		rawInput,
	)
	if err != nil {
		return nil, fmt.Errorf("compra parcelada inválida: %w", err)
	}

	if err := uc.installRepo.SavePurchase(ctx, purchase, installments); err != nil {
		return nil, fmt.Errorf("erro ao salvar compra parcelada: %w", err)
	}

	return &ExpenseOutput{
		ID:                purchase.ID.String(),
		Amount:            purchase.TotalAmount,
		Description:       purchase.Description,
		Category:          string(purchase.Category),
		Payment:           string(purchase.Payment),
		Confidence:        analysis.Confidence,
		Type:              string(ports.ExpenseTypeInstallment),
		TotalInstallments: purchase.TotalInstallments,
		InstallmentAmount: purchase.InstallmentAmount,
	}, nil
}
