package usecase

import (
	"context"
	"fmt"
	"time"

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

	n := analysis.Installments.Total
	totalAmount := *analysis.Amount
	installmentAmount := analysis.Installments.AmountPerInstallment
	if installmentAmount <= 0 {
		installmentAmount = totalAmount / float64(n)
	}

	purchase, err := domain.NewPurchase(
		totalAmount,
		extractDescription(analysis.Description),
		parseCategory(analysis.Category),
		payment,
		domain.PurchaseTypeInstallment,
		rawInput,
	)
	if err != nil {
		return nil, fmt.Errorf("compra parcelada inválida: %w", err)
	}
	purchase.InstallmentCount = &n
	purchase.InstallmentAmount = &installmentAmount

	now := time.Now().UTC()
	payments := make([]domain.Payment, n)
	for i := range n {
		p := domain.NewPayment(purchase.ID, installmentAmount, domain.PaymentStatusPending)
		num := i + 1
		p.InstallmentNumber = &num
		due := now.AddDate(0, i, 0)
		p.DueDate = &due
		payments[i] = *p
	}

	if err := uc.repo.Save(ctx, purchase, payments); err != nil {
		return nil, fmt.Errorf("erro ao salvar compra parcelada: %w", err)
	}

	return &ExpenseOutput{
		ID:                purchase.ID.String(),
		Amount:            purchase.TotalAmount,
		Description:       descriptionOrFallback(purchase.Description, rawInput),
		Category:          purchase.Category.Label(),
		Payment:           purchase.PaymentMethod.Label(),
		Confidence:        analysis.Confidence,
		Type:              string(ports.ExpenseTypeInstallment),
		TotalInstallments: n,
		InstallmentAmount: installmentAmount,
	}, nil
}
