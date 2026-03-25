package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewPayment_Paid(t *testing.T) {
	purchaseID := uuid.New()
	p := NewPayment(purchaseID, 100.0, PaymentStatusPaid)

	if p.PurchaseID != purchaseID {
		t.Errorf("PurchaseID incorreto")
	}
	if p.Amount != 100.0 {
		t.Errorf("amount esperado 100.0, got %v", p.Amount)
	}
	if p.Status != PaymentStatusPaid {
		t.Errorf("status esperado PAID, got %s", p.Status)
	}
	if p.PaidAt == nil {
		t.Error("PaidAt deveria estar preenchido para status PAID")
	}
	if p.ID.String() == "" {
		t.Error("ID não deveria ser vazio")
	}
}

func TestNewPayment_Pending(t *testing.T) {
	p := NewPayment(uuid.New(), 50.0, PaymentStatusPending)

	if p.Status != PaymentStatusPending {
		t.Errorf("status esperado PENDING, got %s", p.Status)
	}
	if p.PaidAt != nil {
		t.Error("PaidAt deveria ser nil para status PENDING")
	}
}

func TestNewPayment_Cancelled(t *testing.T) {
	p := NewPayment(uuid.New(), 50.0, PaymentStatusCancelled)

	if p.Status != PaymentStatusCancelled {
		t.Errorf("status esperado CANCELLED, got %s", p.Status)
	}
	if p.PaidAt != nil {
		t.Error("PaidAt deveria ser nil para status CANCELLED")
	}
}
