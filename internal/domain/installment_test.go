package domain

import (
	"testing"
	"time"
)

func TestNewInstallmentPurchase_Success(t *testing.T) {
	purchase, installments, err := NewInstallmentPurchase(
		"iPhone 15",
		1200.0,
		100.0,
		12,
		CategoryShopping,
		PaymentMethodCreditCard,
		"comprei iPhone em 12x",
	)
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if purchase == nil {
		t.Fatal("purchase não deveria ser nil")
	}
	if purchase.ID == [16]byte{} {
		t.Error("ID não foi gerado")
	}
	if purchase.Description != "iPhone 15" {
		t.Errorf("description esperada 'iPhone 15', got '%s'", purchase.Description)
	}
	if purchase.TotalAmount != 1200.0 {
		t.Errorf("total_amount esperado 1200.0, got %v", purchase.TotalAmount)
	}
	if purchase.InstallmentAmount != 100.0 {
		t.Errorf("installment_amount esperado 100.0, got %v", purchase.InstallmentAmount)
	}
	if purchase.TotalInstallments != 12 {
		t.Errorf("total_installments esperado 12, got %d", purchase.TotalInstallments)
	}
	if purchase.Category != CategoryShopping {
		t.Errorf("category esperada SHOPPING, got %s", purchase.Category)
	}
	if purchase.Payment != PaymentMethodCreditCard {
		t.Errorf("payment esperado CREDIT_CARD, got %s", purchase.Payment)
	}
	if len(installments) != 12 {
		t.Fatalf("esperava 12 installments, got %d", len(installments))
	}
}

func TestNewInstallmentPurchase_InstallmentsAreNumberedSequentially(t *testing.T) {
	_, installments, err := NewInstallmentPurchase(
		"Produto", 300.0, 100.0, 3, CategoryShopping, PaymentMethodCreditCard, "raw",
	)
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}

	for i, inst := range installments {
		expected := i + 1
		if inst.InstallmentNumber != expected {
			t.Errorf("installment[%d].InstallmentNumber esperado %d, got %d", i, expected, inst.InstallmentNumber)
		}
		if inst.TotalInstallments != 3 {
			t.Errorf("installment[%d].TotalInstallments esperado 3, got %d", i, inst.TotalInstallments)
		}
		if inst.Amount != 100.0 {
			t.Errorf("installment[%d].Amount esperado 100.0, got %v", i, inst.Amount)
		}
	}
}

func TestNewInstallmentPurchase_DueDatesAreMonthlyApart(t *testing.T) {
	_, installments, err := NewInstallmentPurchase(
		"Produto", 300.0, 100.0, 3, CategoryShopping, PaymentMethodCreditCard, "raw",
	)
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}

	for i := 1; i < len(installments); i++ {
		prev := installments[i-1].DueDate
		curr := installments[i].DueDate
		diff := curr.Sub(prev)
		// Each installment is approximately 1 month apart (28–31 days)
		if diff < 28*24*time.Hour || diff > 31*24*time.Hour {
			t.Errorf("installment %d due_date diff esperado ~1 mês, got %v", i+1, diff)
		}
	}
}

func TestNewInstallmentPurchase_AllInstallmentsPending(t *testing.T) {
	_, installments, err := NewInstallmentPurchase(
		"Produto", 200.0, 100.0, 2, CategoryOther, PaymentMethodCreditCard, "raw",
	)
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}

	for i, inst := range installments {
		if inst.Status != InstallmentStatusPending {
			t.Errorf("installment[%d].Status esperado PENDING, got %s", i, inst.Status)
		}
		if inst.PaidAt != nil {
			t.Errorf("installment[%d].PaidAt deveria ser nil", i)
		}
	}
}

func TestNewInstallmentPurchase_InvalidAmount(t *testing.T) {
	_, _, err := NewInstallmentPurchase("desc", 0, 0, 3, CategoryOther, PaymentMethodCreditCard, "raw")
	if err != ErrInvalidAmount {
		t.Errorf("esperava ErrInvalidAmount, got %v", err)
	}
}

func TestNewInstallmentPurchase_EmptyDescription(t *testing.T) {
	_, _, err := NewInstallmentPurchase("", 300.0, 100.0, 3, CategoryOther, PaymentMethodCreditCard, "raw")
	if err != ErrEmptyDescription {
		t.Errorf("esperava ErrEmptyDescription, got %v", err)
	}
}

func TestNewInstallmentPurchase_InvalidInstallments(t *testing.T) {
	_, _, err := NewInstallmentPurchase("desc", 300.0, 100.0, 0, CategoryOther, PaymentMethodCreditCard, "raw")
	if err != ErrInvalidInstallments {
		t.Errorf("esperava ErrInvalidInstallments, got %v", err)
	}

	_, _, err = NewInstallmentPurchase("desc", 300.0, 100.0, -1, CategoryOther, PaymentMethodCreditCard, "raw")
	if err != ErrInvalidInstallments {
		t.Errorf("esperava ErrInvalidInstallments para -1, got %v", err)
	}
}

func TestNewInstallmentPurchase_EmptyPayment(t *testing.T) {
	_, _, err := NewInstallmentPurchase("desc", 300.0, 100.0, 3, CategoryOther, "", "raw")
	if err != ErrInvalidPaymentMethod {
		t.Errorf("esperava ErrInvalidPaymentMethod, got %v", err)
	}
}

func TestNewInstallmentPurchase_InstallmentsPurchaseIDMatchesPurchase(t *testing.T) {
	purchase, installments, err := NewInstallmentPurchase(
		"Produto", 300.0, 100.0, 3, CategoryOther, PaymentMethodCreditCard, "raw",
	)
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}

	for i, inst := range installments {
		if inst.PurchaseID != purchase.ID {
			t.Errorf("installment[%d].PurchaseID não corresponde ao purchase.ID", i)
		}
	}
}
