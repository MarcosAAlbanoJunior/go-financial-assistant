package usecase

import (
	"context"
	"log/slog"
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

func TestExecuteText_Recurring_Success(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Amount:        ptr(55.0),
		Description:   ptr("Netflix"),
		Category:      ptr("ENTERTAINMENT"),
		Confidence:    0.99,
		Type:          ports.ExpenseTypeRecurring,
		RecurringInfo: &ports.RecurringInfo{DayOfMonth: 15},
	}

	var savedRecurring *domain.RecurringExpense
	recurringRepo := &mockRecurringRepo{
		saveFn: func(_ context.Context, r *domain.RecurringExpense) error {
			savedRecurring = r
			return nil
		},
	}

	uc := NewAnalyzeExpense(successRepo(), noopInstallRepo(), recurringRepo,
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}}, slog.Default(),
	)

	out, err := uc.ExecuteText(context.Background(), TextInput{Text: "Netflix 55 reais todo mês dia 15"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if out.Type != "RECURRING" {
		t.Errorf("type esperado RECURRING, got %s", out.Type)
	}
	if out.Amount != 55.0 {
		t.Errorf("amount esperado 55.0, got %v", out.Amount)
	}
	if out.DayOfMonth != 15 {
		t.Errorf("day_of_month esperado 15, got %d", out.DayOfMonth)
	}
	if savedRecurring == nil {
		t.Fatal("recurring expense não foi salvo")
	}
	if savedRecurring.Description != "Netflix" {
		t.Errorf("description esperada Netflix, got %s", savedRecurring.Description)
	}
}

func TestExecuteText_Recurring_GeneratesFirstExpense(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Amount:        ptr(55.0),
		Description:   ptr("Netflix"),
		Category:      ptr("ENTERTAINMENT"),
		Type:          ports.ExpenseTypeRecurring,
		RecurringInfo: &ports.RecurringInfo{DayOfMonth: 10},
	}

	var savedExpenses []*domain.Expense
	repo := &mockRepo{
		saveFn: func(_ context.Context, e *domain.Expense) error {
			savedExpenses = append(savedExpenses, e)
			return nil
		},
	}

	uc := NewAnalyzeExpense(repo, noopInstallRepo(), noopRecurringRepo(),
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}}, slog.Default(),
	)

	_, err := uc.ExecuteText(context.Background(), TextInput{Text: "Netflix todo mês"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if len(savedExpenses) != 1 {
		t.Fatalf("esperava 1 despesa gerada, got %d", len(savedExpenses))
	}
	if savedExpenses[0].RecurringExpenseID == nil {
		t.Error("despesa gerada deveria ter RecurringExpenseID preenchido")
	}
}

func TestExecuteText_Recurring_AmountNil(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Amount:      nil,
		Description: ptr("Academia"),
		Type:        ports.ExpenseTypeRecurring,
	}

	uc := NewAnalyzeExpense(successRepo(), noopInstallRepo(), noopRecurringRepo(),
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}}, slog.Default(),
	)

	_, err := uc.ExecuteText(context.Background(), TextInput{Text: "academia todo mês"})
	if err == nil {
		t.Fatal("esperava erro por amount nil")
	}
}

func TestExecuteText_Recurring_DefaultDayOfMonth(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Amount:        ptr(80.0),
		Description:   ptr("Academia"),
		Category:      ptr("HEALTH"),
		Type:          ports.ExpenseTypeRecurring,
		RecurringInfo: nil,
	}

	var savedRecurring *domain.RecurringExpense
	recurringRepo := &mockRecurringRepo{
		saveFn: func(_ context.Context, r *domain.RecurringExpense) error {
			savedRecurring = r
			return nil
		},
	}

	uc := NewAnalyzeExpense(successRepo(), noopInstallRepo(), recurringRepo,
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}}, slog.Default(),
	)

	out, err := uc.ExecuteText(context.Background(), TextInput{Text: "academia 80 por mês"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if out.DayOfMonth != 1 {
		t.Errorf("day_of_month padrão esperado 1, got %d", out.DayOfMonth)
	}
	if savedRecurring.DayOfMonth != 1 {
		t.Errorf("saved DayOfMonth esperado 1, got %d", savedRecurring.DayOfMonth)
	}
}

// ---- cancel recurring ----

func TestExecuteText_CancelRecurring_Success(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Confidence: 0.95,
		Type:       ports.ExpenseTypeCancelRecurring,
		CancelInfo: &ports.CancelInfo{Description: "Netflix"},
	}

	existing := domain.RecurringExpense{
		ID:          uuid.New(),
		Description: "Netflix",
		Amount:      55.0,
		Category:    domain.CategoryEntertainment,
		Payment:     domain.PaymentMethodCreditCard,
		DayOfMonth:  15,
		IsActive:    true,
	}

	var updated *domain.RecurringExpense
	recurringRepo := &mockRecurringRepo{
		findByDescriptionFn: func(_ context.Context, _ string) ([]domain.RecurringExpense, error) {
			return []domain.RecurringExpense{existing}, nil
		},
		updateFn: func(_ context.Context, r *domain.RecurringExpense) error {
			updated = r
			return nil
		},
	}

	uc := NewAnalyzeExpense(successRepo(), noopInstallRepo(), recurringRepo,
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}}, slog.Default(),
	)

	out, err := uc.ExecuteText(context.Background(), TextInput{Text: "cancelei Netflix"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if out.Type != "CANCEL_RECURRING" {
		t.Errorf("type esperado CANCEL_RECURRING, got %s", out.Type)
	}
	if !out.Cancelled {
		t.Error("cancelled deveria ser true")
	}
	if out.CancelledDescription != "Netflix" {
		t.Errorf("cancelled_description esperado Netflix, got %s", out.CancelledDescription)
	}
	if updated == nil {
		t.Fatal("recurring não foi atualizado")
	}
	if updated.IsActive {
		t.Error("recurring deveria estar inativo após cancelamento")
	}
	if updated.CancelledAt == nil {
		t.Error("CancelledAt deveria estar preenchido")
	}
}

func TestExecuteText_CancelRecurring_NotFound(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Type:       ports.ExpenseTypeCancelRecurring,
		CancelInfo: &ports.CancelInfo{Description: "ServicoInexistente"},
	}

	recurringRepo := &mockRecurringRepo{
		findByDescriptionFn: func(_ context.Context, _ string) ([]domain.RecurringExpense, error) {
			return []domain.RecurringExpense{}, nil
		},
	}

	uc := NewAnalyzeExpense(successRepo(), noopInstallRepo(), recurringRepo,
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}}, slog.Default(),
	)

	_, err := uc.ExecuteText(context.Background(), TextInput{Text: "cancelei algo"})
	if err == nil {
		t.Fatal("esperava erro por não encontrar despesa recorrente")
	}
}

func TestExecuteText_CancelRecurring_NoDescription(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Type:       ports.ExpenseTypeCancelRecurring,
		CancelInfo: nil,
	}

	uc := NewAnalyzeExpense(successRepo(), noopInstallRepo(), noopRecurringRepo(),
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}}, slog.Default(),
	)

	_, err := uc.ExecuteText(context.Background(), TextInput{Text: "cancelei algo"})
	if err == nil {
		t.Fatal("esperava erro por falta de descrição")
	}
}

func TestExecuteText_CancelRecurring_UsesDescriptionFieldAsFallback(t *testing.T) {
	// CancelInfo is nil but Description is populated — should use it as fallback.
	analysis := &ports.ExpenseAnalysis{
		Type:        ports.ExpenseTypeCancelRecurring,
		Description: ptr("Spotify"),
		CancelInfo:  nil,
	}

	existing := domain.RecurringExpense{
		ID: uuid.New(), Description: "Spotify", Amount: 20.0,
		Category: domain.CategoryEntertainment, Payment: domain.PaymentMethodCreditCard, IsActive: true,
	}

	recurringRepo := &mockRecurringRepo{
		findByDescriptionFn: func(_ context.Context, _ string) ([]domain.RecurringExpense, error) {
			return []domain.RecurringExpense{existing}, nil
		},
	}

	uc := NewAnalyzeExpense(successRepo(), noopInstallRepo(), recurringRepo,
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}}, slog.Default(),
	)

	out, err := uc.ExecuteText(context.Background(), TextInput{Text: "cancelei Spotify"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if out.CancelledDescription != "Spotify" {
		t.Errorf("esperava descrição Spotify, got %s", out.CancelledDescription)
	}
}
