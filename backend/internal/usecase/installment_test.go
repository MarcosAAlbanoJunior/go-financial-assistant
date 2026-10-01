package usecase

import (
	"context"
	"errors"
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

	var savedPurchase *domain.Purchase
	var savedPayments []domain.Payment

	repo := &mockPurchaseRepo{
		saveFn: func(_ context.Context, p *domain.Purchase, pmts []domain.Payment) error {
			savedPurchase = p
			savedPayments = pmts
			return nil
		},
	}

	uc := newUC(repo, &mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
		return analysis, nil
	}})

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
	if savedPurchase.Type != domain.PurchaseTypeInstallment {
		t.Errorf("type esperado INSTALLMENT, got %s", savedPurchase.Type)
	}
	if len(savedPayments) != 12 {
		t.Errorf("esperava 12 pagamentos salvos, got %d", len(savedPayments))
	}
}

func TestExecuteText_Installment_AmountNil(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Amount:       nil,
		Description:  ptr("Produto"),
		Type:         ports.ExpenseTypeInstallment,
		Installments: &ports.InstallmentInfo{Total: 6, AmountPerInstallment: 50},
	}

	uc := newUC(successRepo(), &mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
		return analysis, nil
	}})

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

	repo := &mockPurchaseRepo{
		saveFn: func(_ context.Context, _ *domain.Purchase, _ []domain.Payment) error {
			return errors.New("db error")
		},
	}

	uc := newUC(repo, &mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
		return analysis, nil
	}})

	_, err := uc.ExecuteText(context.Background(), TextInput{Text: "compra parcelada"})
	if err == nil {
		t.Fatal("esperava erro do repo")
	}
}

func TestExecuteText_Installment_InstallmentAmountCalculated_WhenZero(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Amount:       ptr(300.0),
		Description:  ptr("Produto"),
		Category:     ptr("SHOPPING"),
		Confidence:   0.9,
		Type:         ports.ExpenseTypeInstallment,
		Installments: &ports.InstallmentInfo{Total: 3, AmountPerInstallment: 0},
	}

	var savedPurchase *domain.Purchase
	repo := &mockPurchaseRepo{
		saveFn: func(_ context.Context, p *domain.Purchase, _ []domain.Payment) error {
			savedPurchase = p
			return nil
		},
	}

	uc := newUC(repo, &mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
		return analysis, nil
	}})

	_, err := uc.ExecuteText(context.Background(), TextInput{Text: "compra em 3x"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if savedPurchase.InstallmentAmount == nil || *savedPurchase.InstallmentAmount != 100.0 {
		var got interface{}
		if savedPurchase.InstallmentAmount != nil {
			got = *savedPurchase.InstallmentAmount
		}
		t.Errorf("installment_amount calculado esperado 100.0, got %v", got)
	}
}

func TestExecuteText_Installment_PaymentsHaveDueDates(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Amount:       ptr(300.0),
		Description:  ptr("Notebook"),
		Category:     ptr("SHOPPING"),
		Confidence:   0.9,
		Type:         ports.ExpenseTypeInstallment,
		Installments: &ports.InstallmentInfo{Total: 3, AmountPerInstallment: 100},
	}

	var savedPayments []domain.Payment
	repo := &mockPurchaseRepo{
		saveFn: func(_ context.Context, _ *domain.Purchase, pmts []domain.Payment) error {
			savedPayments = pmts
			return nil
		},
	}

	uc := newUC(repo, &mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
		return analysis, nil
	}})

	_, err := uc.ExecuteText(context.Background(), TextInput{Text: "notebook 3x"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	for i, p := range savedPayments {
		if p.DueDate == nil {
			t.Errorf("pagamento %d sem due_date", i+1)
		}
		if p.InstallmentNumber == nil || *p.InstallmentNumber != i+1 {
			t.Errorf("parcela %d com installment_number errado", i+1)
		}
		if p.Status != domain.PaymentStatusPending {
			t.Errorf("parcela %d esperava status PENDING, got %s", i+1, p.Status)
		}
	}
}
