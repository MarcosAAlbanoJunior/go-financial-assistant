package domain

import (
	"testing"
	"time"
)

func TestNewRecurringExpense_Success(t *testing.T) {
	r, err := NewRecurringExpense(
		"Netflix",
		55.0,
		CategoryEntertainment,
		PaymentMethodCreditCard,
		15,
		"Netflix 55 reais todo mês",
	)
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if r.ID == [16]byte{} {
		t.Error("ID não foi gerado")
	}
	if r.Description != "Netflix" {
		t.Errorf("description esperada Netflix, got %s", r.Description)
	}
	if r.Amount != 55.0 {
		t.Errorf("amount esperado 55.0, got %v", r.Amount)
	}
	if r.Category != CategoryEntertainment {
		t.Errorf("category esperada ENTERTAINMENT, got %s", r.Category)
	}
	if r.Payment != PaymentMethodCreditCard {
		t.Errorf("payment esperado CREDIT_CARD, got %s", r.Payment)
	}
	if r.DayOfMonth != 15 {
		t.Errorf("day_of_month esperado 15, got %d", r.DayOfMonth)
	}
	if !r.IsActive {
		t.Error("IsActive deveria ser true")
	}
	if r.CancelledAt != nil {
		t.Error("CancelledAt deveria ser nil")
	}
	if r.LastGeneratedDate != nil {
		t.Error("LastGeneratedDate deveria ser nil")
	}
}

func TestNewRecurringExpense_InvalidAmount(t *testing.T) {
	_, err := NewRecurringExpense("desc", 0, CategoryOther, PaymentMethodOther, 1, "raw")
	if err != ErrInvalidAmount {
		t.Errorf("esperava ErrInvalidAmount, got %v", err)
	}

	_, err = NewRecurringExpense("desc", -10, CategoryOther, PaymentMethodOther, 1, "raw")
	if err != ErrInvalidAmount {
		t.Errorf("esperava ErrInvalidAmount para valor negativo, got %v", err)
	}
}

func TestNewRecurringExpense_EmptyDescription(t *testing.T) {
	_, err := NewRecurringExpense("", 50.0, CategoryOther, PaymentMethodOther, 1, "raw")
	if err != ErrEmptyDescription {
		t.Errorf("esperava ErrEmptyDescription, got %v", err)
	}
}

func TestNewRecurringExpense_EmptyPayment(t *testing.T) {
	_, err := NewRecurringExpense("desc", 50.0, CategoryOther, "", 1, "raw")
	if err != ErrInvalidPaymentMethod {
		t.Errorf("esperava ErrInvalidPaymentMethod, got %v", err)
	}
}

func TestNewRecurringExpense_DayOfMonth_OutOfRange_DefaultsToOne(t *testing.T) {
	cases := []int{0, -1, 29, 31, 100}
	for _, day := range cases {
		r, err := NewRecurringExpense("desc", 50.0, CategoryOther, PaymentMethodOther, day, "raw")
		if err != nil {
			t.Fatalf("day=%d: esperava sucesso, got %v", day, err)
		}
		if r.DayOfMonth != 1 {
			t.Errorf("day=%d: DayOfMonth esperado 1, got %d", day, r.DayOfMonth)
		}
	}
}

func TestNewRecurringExpense_DayOfMonth_ValidRange(t *testing.T) {
	for _, day := range []int{1, 15, 28} {
		r, err := NewRecurringExpense("desc", 50.0, CategoryOther, PaymentMethodOther, day, "raw")
		if err != nil {
			t.Fatalf("day=%d: esperava sucesso, got %v", day, err)
		}
		if r.DayOfMonth != day {
			t.Errorf("day=%d: DayOfMonth esperado %d, got %d", day, day, r.DayOfMonth)
		}
	}
}

func TestRecurringExpense_Cancel(t *testing.T) {
	r, _ := NewRecurringExpense("Netflix", 55.0, CategoryEntertainment, PaymentMethodCreditCard, 15, "raw")

	r.Cancel("não uso mais")

	if r.IsActive {
		t.Error("IsActive deveria ser false após cancelamento")
	}
	if r.CancelledAt == nil {
		t.Error("CancelledAt deveria estar preenchido")
	}
	if r.CancellationReason == nil || *r.CancellationReason != "não uso mais" {
		t.Errorf("CancellationReason esperado 'não uso mais', got %v", r.CancellationReason)
	}
	if r.EndDate == nil {
		t.Error("EndDate deveria estar preenchido")
	}
}

func TestRecurringExpense_GenerateExpense(t *testing.T) {
	r, _ := NewRecurringExpense("Netflix", 55.0, CategoryEntertainment, PaymentMethodCreditCard, 15, "Netflix raw")

	expense, err := r.GenerateExpense()
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if expense.Amount != 55.0 {
		t.Errorf("amount esperado 55.0, got %v", expense.Amount)
	}
	if expense.Description != "Netflix" {
		t.Errorf("description esperada Netflix, got %s", expense.Description)
	}
	if expense.Category != CategoryEntertainment {
		t.Errorf("category esperada ENTERTAINMENT, got %s", expense.Category)
	}
	if expense.RecurringExpenseID == nil || *expense.RecurringExpenseID != r.ID {
		t.Error("RecurringExpenseID deveria apontar para o ID da recorrência")
	}
}

func TestRecurringExpense_ShouldGenerateForMonth(t *testing.T) {
	r, _ := NewRecurringExpense("Netflix", 55.0, CategoryEntertainment, PaymentMethodCreditCard, 15, "raw")

	// No last generated date → should generate
	if !r.ShouldGenerateForMonth(2026, time.March) {
		t.Error("deveria gerar quando LastGeneratedDate é nil")
	}

	// Set last generated to March 2026 → should NOT generate for March again
	march := time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC)
	r.LastGeneratedDate = &march

	if r.ShouldGenerateForMonth(2026, time.March) {
		t.Error("não deveria gerar novamente em março 2026")
	}

	// Should generate for April 2026
	if !r.ShouldGenerateForMonth(2026, time.April) {
		t.Error("deveria gerar em abril 2026")
	}

	// Should generate for 2027
	if !r.ShouldGenerateForMonth(2027, time.January) {
		t.Error("deveria gerar em 2027")
	}
}

func TestRecurringExpense_ShouldGenerateForMonth_Inactive(t *testing.T) {
	r, _ := NewRecurringExpense("Netflix", 55.0, CategoryEntertainment, PaymentMethodCreditCard, 15, "raw")
	r.Cancel("não uso mais")

	if r.ShouldGenerateForMonth(2026, time.March) {
		t.Error("não deveria gerar quando inativo")
	}
}
