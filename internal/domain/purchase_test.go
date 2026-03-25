package domain

import (
	"testing"
)

func TestNewPurchase_Valid(t *testing.T) {
	desc := "Supermercado"
	p, err := NewPurchase(150.0, &desc, CategoryFood, PaymentMethodPix, PurchaseTypeSingle, "compra no pix")
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if p.TotalAmount != 150.0 {
		t.Errorf("amount esperado 150.0, got %v", p.TotalAmount)
	}
	if p.Description == nil || *p.Description != "Supermercado" {
		t.Errorf("description esperada 'Supermercado', got %v", p.Description)
	}
	if !p.IsActive {
		t.Error("purchase novo deveria estar ativo")
	}
	if p.ID.String() == "" {
		t.Error("ID não deveria ser vazio")
	}
}

func TestNewPurchase_NilDescription(t *testing.T) {
	p, err := NewPurchase(50.0, nil, CategoryOther, PaymentMethodCash, PurchaseTypeSingle, "raw")
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if p.Description != nil {
		t.Errorf("description deveria ser nil, got %v", p.Description)
	}
}

func TestNewPurchase_InvalidAmount(t *testing.T) {
	_, err := NewPurchase(0, nil, CategoryOther, PaymentMethodCash, PurchaseTypeSingle, "raw")
	if err == nil {
		t.Fatal("esperava erro para amount zero")
	}
	if err != ErrInvalidAmount {
		t.Errorf("esperava ErrInvalidAmount, got: %v", err)
	}

	_, err = NewPurchase(-10, nil, CategoryOther, PaymentMethodCash, PurchaseTypeSingle, "raw")
	if err == nil {
		t.Fatal("esperava erro para amount negativo")
	}
}

func TestPurchase_Cancel(t *testing.T) {
	p, _ := NewPurchase(100.0, nil, CategoryOther, PaymentMethodPix, PurchaseTypeSingle, "raw")

	p.Cancel("teste de cancelamento")

	if p.IsActive {
		t.Error("purchase deveria estar inativo após cancelamento")
	}
	if p.CancelledAt == nil {
		t.Error("CancelledAt deveria estar preenchido")
	}
	if p.CancellationReason == nil || *p.CancellationReason != "teste de cancelamento" {
		t.Errorf("CancellationReason esperado 'teste de cancelamento', got %v", p.CancellationReason)
	}
}

func TestNewPurchase_Recurring_Types(t *testing.T) {
	p, err := NewPurchase(80.0, nil, CategoryHealth, PaymentMethodCreditCard, PurchaseTypeRecurring, "raw")
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if p.Type != PurchaseTypeRecurring {
		t.Errorf("type esperado RECURRING, got %s", p.Type)
	}
}
