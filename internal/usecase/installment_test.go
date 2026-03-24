package usecase

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func TestExecuteText_Installment_Success(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Amount:       ptr(1200.0),
		Description:  ptr("iPhone 15"),
		Category:     ptr("SHOPPING"),
		Confidence:   0.97,
		Type:         ports.ExpenseTypeInstallment,
		Installments: &ports.InstallmentInfo{Total: 12, AmountPerInstallment: 100.0},
	}

	var savedPurchase *domain.InstallmentPurchase
	var savedInstallments []domain.Installment

	installRepo := &mockInstallRepo{
		savePurchaseFn: func(_ context.Context, p *domain.InstallmentPurchase, insts []domain.Installment) error {
			savedPurchase = p
			savedInstallments = insts
			return nil
		},
	}

	uc := NewAnalyzeExpense(successRepo(), installRepo, noopRecurringRepo(),
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}}, slog.Default(),
	)

	out, err := uc.ExecuteText(context.Background(), TextInput{Text: "comprei iPhone 15 em 12x de 100"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if out.Type != "INSTALLMENT" {
		t.Errorf("type esperado INSTALLMENT, got %s", out.Type)
	}
	if out.Amount != 1200.0 {
		t.Errorf("amount esperado 1200.0, got %v", out.Amount)
	}
	if out.TotalInstallments != 12 {
		t.Errorf("total_installments esperado 12, got %d", out.TotalInstallments)
	}
	if out.InstallmentAmount != 100.0 {
		t.Errorf("installment_amount esperado 100.0, got %v", out.InstallmentAmount)
	}
	if savedPurchase == nil {
		t.Fatal("purchase não foi salvo")
	}
	if len(savedInstallments) != 12 {
		t.Errorf("esperava 12 parcelas salvas, got %d", len(savedInstallments))
	}
}

func TestExecuteText_Installment_AmountNil(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Amount:       nil,
		Description:  ptr("Produto"),
		Type:         ports.ExpenseTypeInstallment,
		Installments: &ports.InstallmentInfo{Total: 6, AmountPerInstallment: 50},
	}

	uc := NewAnalyzeExpense(successRepo(), noopInstallRepo(), noopRecurringRepo(),
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}}, slog.Default(),
	)

	_, err := uc.ExecuteText(context.Background(), TextInput{Text: "comprei parcelado"})
	if err == nil {
		t.Fatal("esperava erro por amount nil")
	}
}

func TestExecuteText_Installment_NoInstallmentInfo_FallbackToSingle(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Amount:       ptr(300.0),
		Description:  ptr("Produto"),
		Category:     ptr("SHOPPING"),
		Confidence:   0.8,
		Type:         ports.ExpenseTypeInstallment,
		Installments: nil,
	}

	uc := newUC(
		successRepo(),
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}},
	)

	out, err := uc.ExecuteText(context.Background(), TextInput{Text: "comprei algo"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if out.Type != "SINGLE" {
		t.Errorf("type esperado SINGLE após fallback, got %s", out.Type)
	}
}

func TestExecuteText_Installment_RepoError(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Amount:       ptr(600.0),
		Description:  ptr("Produto"),
		Category:     ptr("SHOPPING"),
		Type:         ports.ExpenseTypeInstallment,
		Installments: &ports.InstallmentInfo{Total: 6, AmountPerInstallment: 100},
	}

	installRepo := &mockInstallRepo{
		savePurchaseFn: func(_ context.Context, _ *domain.InstallmentPurchase, _ []domain.Installment) error {
			return errors.New("db error")
		},
	}

	uc := NewAnalyzeExpense(successRepo(), installRepo, noopRecurringRepo(),
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}}, slog.Default(),
	)

	_, err := uc.ExecuteText(context.Background(), TextInput{Text: "compra parcelada"})
	if err == nil {
		t.Fatal("esperava erro do repo")
	}
}

func TestExecuteText_Installment_InstallmentAmountCalculated_WhenZero(t *testing.T) {
	// When amount_per_installment is 0, should be calculated from total / total_installments.
	analysis := &ports.ExpenseAnalysis{
		Amount:       ptr(300.0),
		Description:  ptr("Produto"),
		Category:     ptr("SHOPPING"),
		Confidence:   0.9,
		Type:         ports.ExpenseTypeInstallment,
		Installments: &ports.InstallmentInfo{Total: 3, AmountPerInstallment: 0},
	}

	var savedPurchase *domain.InstallmentPurchase
	installRepo := &mockInstallRepo{
		savePurchaseFn: func(_ context.Context, p *domain.InstallmentPurchase, _ []domain.Installment) error {
			savedPurchase = p
			return nil
		},
	}

	uc := NewAnalyzeExpense(successRepo(), installRepo, noopRecurringRepo(),
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}}, slog.Default(),
	)

	_, err := uc.ExecuteText(context.Background(), TextInput{Text: "compra em 3x"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if savedPurchase.InstallmentAmount != 100.0 {
		t.Errorf("installment_amount calculado esperado 100.0, got %v", savedPurchase.InstallmentAmount)
	}
}
