package domain

import (
	"testing"
)

func TestNewExpense_Success(t *testing.T) {
	url := "https://example.com/receipt.jpg"
	expense, err := NewExpense(100.0, "Almoço", CategoryFood, PaymentMethodPix, "texto bruto", &url)
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if expense.ID == [16]byte{} {
		t.Error("ID não foi gerado")
	}
	if expense.Amount != 100.0 {
		t.Errorf("amount esperado 100.0, got %v", expense.Amount)
	}
	if expense.Description != "Almoço" {
		t.Errorf("description esperada 'Almoço', got '%s'", expense.Description)
	}
	if expense.Category != CategoryFood {
		t.Errorf("category esperada FOOD, got %s", expense.Category)
	}
	if expense.Payment != PaymentMethodPix {
		t.Errorf("payment esperado PIX, got %s", expense.Payment)
	}
	if expense.RawInput != "texto bruto" {
		t.Errorf("rawInput esperado 'texto bruto', got '%s'", expense.RawInput)
	}
	if expense.ReceiptURL != &url {
		t.Error("receiptURL não foi atribuído corretamente")
	}
	if expense.CreatedAt.IsZero() {
		t.Error("createdAt não foi preenchido")
	}
}

func TestNewExpense_NilReceiptURL(t *testing.T) {
	expense, err := NewExpense(50.0, "Taxi", CategoryTransport, PaymentMethodCash, "raw", nil)
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if expense.ReceiptURL != nil {
		t.Error("receiptURL deveria ser nil")
	}
}

func TestNewExpense_InvalidAmount(t *testing.T) {
	cases := []float64{0, -1, -100}
	for _, amount := range cases {
		_, err := NewExpense(amount, "desc", CategoryOther, PaymentMethodOther, "raw", nil)
		if err == nil {
			t.Errorf("amount %v: esperava erro", amount)
		}
		if err != ErrInvalidAmount {
			t.Errorf("amount %v: esperava ErrInvalidAmount, got %v", amount, err)
		}
	}
}

func TestNewExpense_EmptyDescription(t *testing.T) {
	_, err := NewExpense(10.0, "", CategoryOther, PaymentMethodOther, "raw", nil)
	if err == nil {
		t.Fatal("esperava erro")
	}
	if err != ErrEmptyDescription {
		t.Errorf("esperava ErrEmptyDescription, got %v", err)
	}
}

func TestNewExpense_EmptyPaymentMethod(t *testing.T) {
	_, err := NewExpense(10.0, "desc", CategoryOther, "", "raw", nil)
	if err == nil {
		t.Fatal("esperava erro")
	}
	if err != ErrInvalidPaymentMethod {
		t.Errorf("esperava ErrInvalidPaymentMethod, got %v", err)
	}
}
