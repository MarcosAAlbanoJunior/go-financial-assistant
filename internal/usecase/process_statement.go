package usecase

import (
	"context"
	"fmt"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func (uc *AnalyzeExpense) ExecuteDocument(ctx context.Context, input DocumentInput) (*StatementOutput, error) {
	analysis, err := uc.analyzer.AnalyzeDocument(ctx, input.Data, input.MimeType)
	if err != nil {
		return nil, fmt.Errorf("erro ao analisar extrato: %w", err)
	}

	output := &StatementOutput{}

	for _, tx := range analysis.Transactions {
		if tx.Amount <= 0 {
			continue
		}

		isDuplicate, err := uc.repo.ExistsPaymentByDateAndAmount(ctx, tx.Date, tx.Amount)
		if err != nil {
			uc.logger.Error("erro ao verificar duplicata", "description", tx.RawDescription, "error", err)
			continue
		}

		if isDuplicate {
			output.Pending = append(output.Pending, PendingTransaction{
				Date:        tx.Date,
				Description: tx.Description,
				Amount:      tx.Amount,
				Category:    tx.Category,
				Payment:     tx.PaymentMethod,
				RawInput:    fmt.Sprintf("[extrato: %s]", tx.RawDescription),
			})
			continue
		}

		if err := uc.saveStatementTransaction(ctx, tx); err != nil {
			uc.logger.Error("erro ao salvar transação do extrato", "description", tx.RawDescription, "error", err)
			continue
		}

		output.Inserted++
	}

	return output, nil
}

func (uc *AnalyzeExpense) SavePendingTransaction(ctx context.Context, tx PendingTransaction) error {
	payment, err := parsePaymentMethod(tx.Payment)
	if err != nil {
		payment = domain.PaymentMethodOther
	}

	purchase, err := domain.NewPurchase(
		tx.Amount,
		&tx.Description,
		parseCategory(categoryPtr(tx.Category)),
		payment,
		domain.PurchaseTypeSingle,
		tx.RawInput,
	)
	if err != nil {
		return fmt.Errorf("despesa inválida: %w", err)
	}

	pmt := domain.NewPayment(purchase.ID, tx.Amount, domain.PaymentStatusPaid)
	pmt.DueDate = &tx.Date
	pmt.PaidAt = &tx.Date

	return uc.repo.Save(ctx, purchase, []domain.Payment{*pmt})
}

func categoryPtr(s string) *string { return &s }

func (uc *AnalyzeExpense) saveStatementTransaction(ctx context.Context, tx ports.StatementTransaction) error {
	payment, err := parsePaymentMethod(tx.PaymentMethod)
	if err != nil {
		payment = domain.PaymentMethodOther
	}

	desc := tx.Description
	purchase, err := domain.NewPurchase(
		tx.Amount,
		&desc,
		parseCategory(&tx.Category),
		payment,
		domain.PurchaseTypeSingle,
		fmt.Sprintf("[extrato: %s]", tx.RawDescription),
	)
	if err != nil {
		return fmt.Errorf("despesa inválida: %w", err)
	}

	pmt := domain.NewPayment(purchase.ID, tx.Amount, domain.PaymentStatusPaid)
	pmt.DueDate = &tx.Date
	pmt.PaidAt = &tx.Date

	return uc.repo.Save(ctx, purchase, []domain.Payment{*pmt})
}
