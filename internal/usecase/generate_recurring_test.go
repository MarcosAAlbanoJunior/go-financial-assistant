package usecase

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

func recurringPurchase(dayOfMonth int) domain.Purchase {
	day := dayOfMonth
	desc := "Netflix"
	return domain.Purchase{
		ID:            uuid.New(),
		Description:   &desc,
		TotalAmount:   55.0,
		Category:      domain.CategoryEntertainment,
		PaymentMethod: domain.PaymentMethodCreditCard,
		Type:          domain.PurchaseTypeRecurring,
		IsActive:      true,
		DayOfMonth:    &day,
		RawInput:      "Netflix todo mês",
	}
}

func TestGenerateRecurringExpenses_SkipsWhenNotTargetDay(t *testing.T) {
	today := time.Now().UTC().Day()
	// configura dia diferente do hoje
	differentDay := today%28 + 1
	if differentDay == today {
		differentDay = differentDay%28 + 1
	}

	paymentSaved := false
	repo := &mockPurchaseRepo{
		findActiveRecurringFn: func(_ context.Context) ([]domain.Purchase, error) {
			return []domain.Purchase{recurringPurchase(differentDay)}, nil
		},
		savePaymentFn: func(_ context.Context, _ *domain.Payment) error {
			paymentSaved = true
			return nil
		},
	}

	uc := NewAnalyzeExpense(repo, &mockAnalyzer{}, slog.Default())
	if err := uc.GenerateRecurringExpenses(context.Background()); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if paymentSaved {
		t.Error("não deveria gerar pagamento fora do dia configurado")
	}
}

func TestGenerateRecurringExpenses_GeneratesOnTargetDay(t *testing.T) {
	today := time.Now().UTC().Day()

	paymentSaved := false
	repo := &mockPurchaseRepo{
		findActiveRecurringFn: func(_ context.Context) ([]domain.Purchase, error) {
			return []domain.Purchase{recurringPurchase(today)}, nil
		},
		hasPaymentForMonthFn: func(_ context.Context, _ uuid.UUID, _ time.Time) (bool, error) {
			return false, nil
		},
		savePaymentFn: func(_ context.Context, _ *domain.Payment) error {
			paymentSaved = true
			return nil
		},
	}

	uc := NewAnalyzeExpense(repo, &mockAnalyzer{}, slog.Default())
	if err := uc.GenerateRecurringExpenses(context.Background()); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !paymentSaved {
		t.Error("deveria ter gerado o pagamento no dia configurado")
	}
}

func TestGenerateRecurringExpenses_SkipsAlreadyPaidMonth(t *testing.T) {
	today := time.Now().UTC().Day()

	paymentSaved := false
	repo := &mockPurchaseRepo{
		findActiveRecurringFn: func(_ context.Context) ([]domain.Purchase, error) {
			return []domain.Purchase{recurringPurchase(today)}, nil
		},
		hasPaymentForMonthFn: func(_ context.Context, _ uuid.UUID, _ time.Time) (bool, error) {
			return true, nil // já tem pagamento este mês
		},
		savePaymentFn: func(_ context.Context, _ *domain.Payment) error {
			paymentSaved = true
			return nil
		},
	}

	uc := NewAnalyzeExpense(repo, &mockAnalyzer{}, slog.Default())
	if err := uc.GenerateRecurringExpenses(context.Background()); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if paymentSaved {
		t.Error("não deveria gerar pagamento duplicado no mesmo mês")
	}
}

func TestLastValidDay_NormalDay(t *testing.T) {
	got := lastValidDay(2025, time.March, 15)
	if got != 15 {
		t.Errorf("esperava 15, got %d", got)
	}
}

func TestLastValidDay_DayExceedsMonth(t *testing.T) {
	// Fevereiro 2025 tem 28 dias
	got := lastValidDay(2025, time.February, 31)
	if got != 28 {
		t.Errorf("esperava 28 (último dia de fev/2025), got %d", got)
	}
}

func TestLastValidDay_LeapYear(t *testing.T) {
	// Fevereiro 2024 tem 29 dias (ano bissexto)
	got := lastValidDay(2024, time.February, 30)
	if got != 29 {
		t.Errorf("esperava 29 (último dia de fev/2024 bissexto), got %d", got)
	}
}

func TestLastValidDay_Day31InMonth30(t *testing.T) {
	// Abril tem 30 dias
	got := lastValidDay(2025, time.April, 31)
	if got != 30 {
		t.Errorf("esperava 30 (último dia de abril), got %d", got)
	}
}
