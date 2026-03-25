package usecase

import (
	"context"
	"testing"
	"time"

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
	if savedPurchase == nil {
		t.Fatal("purchase não foi salvo")
	}
	if savedPurchase.Description == nil || *savedPurchase.Description != "Netflix" {
		t.Errorf("description esperada Netflix, got %v", savedPurchase.Description)
	}
	if savedPurchase.DayOfMonth == nil || *savedPurchase.DayOfMonth != 15 {
		t.Errorf("day_of_month esperado 15 no purchase salvo")
	}
	if len(savedPayments) != 1 {
		t.Fatalf("esperava 1 pagamento inicial, got %d", len(savedPayments))
	}
	if savedPayments[0].ReferenceMonth == nil {
		t.Error("primeiro pagamento deveria ter reference_month preenchido")
	}
}

func TestExecuteText_Recurring_GeneratesFirstPayment(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Amount:        ptr(55.0),
		Description:   ptr("Netflix"),
		Category:      ptr("ENTERTAINMENT"),
		Type:          ports.ExpenseTypeRecurring,
		RecurringInfo: &ports.RecurringInfo{DayOfMonth: 10},
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

	_, err := uc.ExecuteText(context.Background(), TextInput{Text: "Netflix todo mês"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if len(savedPayments) != 1 {
		t.Fatalf("esperava 1 pagamento gerado, got %d", len(savedPayments))
	}
	if savedPayments[0].Status != domain.PaymentStatusPaid {
		t.Errorf("primeiro pagamento deveria ter status PAID, got %s", savedPayments[0].Status)
	}
	if savedPayments[0].ReferenceMonth == nil {
		t.Error("primeiro pagamento deveria ter reference_month preenchido")
	}
}

func TestExecuteText_Recurring_AmountNil(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Amount:      nil,
		Description: ptr("Academia"),
		Type:        ports.ExpenseTypeRecurring,
	}

	uc := newUC(successRepo(), &mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
		return analysis, nil
	}})

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

	expectedDay := time.Now().UTC().Day()

	out, err := uc.ExecuteText(context.Background(), TextInput{Text: "academia 80 por mês"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if out.DayOfMonth != expectedDay {
		t.Errorf("day_of_month padrão esperado %d (hoje), got %d", expectedDay, out.DayOfMonth)
	}
	if savedPurchase.DayOfMonth == nil || *savedPurchase.DayOfMonth != expectedDay {
		t.Errorf("purchase salvo deveria ter day_of_month = %d", expectedDay)
	}
}

func TestExecuteText_CancelRecurring_Success(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Confidence: 0.95,
		Type:       ports.ExpenseTypeCancelRecurring,
		CancelInfo: &ports.CancelInfo{Description: "Netflix"},
	}

	desc := "Netflix"
	existing := domain.Purchase{
		ID:            uuid.New(),
		Description:   &desc,
		TotalAmount:   55.0,
		Category:      domain.CategoryEntertainment,
		PaymentMethod: domain.PaymentMethodCreditCard,
		Type:          domain.PurchaseTypeRecurring,
		IsActive:      true,
		RawInput:      "Netflix todo mês",
	}

	var updated *domain.Purchase
	repo := &mockPurchaseRepo{
		findByDescriptionFn: func(_ context.Context, _ string) ([]domain.Purchase, error) {
			return []domain.Purchase{existing}, nil
		},
		updateFn: func(_ context.Context, p *domain.Purchase) error {
			updated = p
			return nil
		},
	}

	uc := newUC(repo, &mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
		return analysis, nil
	}})

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
		t.Fatal("purchase não foi atualizado")
	}
	if updated.IsActive {
		t.Error("purchase deveria estar inativo após cancelamento")
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

	repo := &mockPurchaseRepo{
		findByDescriptionFn: func(_ context.Context, _ string) ([]domain.Purchase, error) {
			return []domain.Purchase{}, nil
		},
	}

	uc := newUC(repo, &mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
		return analysis, nil
	}})

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

	uc := newUC(successRepo(), &mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
		return analysis, nil
	}})

	_, err := uc.ExecuteText(context.Background(), TextInput{Text: "cancelei algo"})
	if err == nil {
		t.Fatal("esperava erro por falta de descrição")
	}
}

func TestExecuteText_CancelRecurring_UsesDescriptionFieldAsFallback(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{
		Type:        ports.ExpenseTypeCancelRecurring,
		Description: ptr("Spotify"),
		CancelInfo:  nil,
	}

	desc := "Spotify"
	existing := domain.Purchase{
		ID:            uuid.New(),
		Description:   &desc,
		TotalAmount:   20.0,
		Category:      domain.CategoryEntertainment,
		PaymentMethod: domain.PaymentMethodCreditCard,
		Type:          domain.PurchaseTypeRecurring,
		IsActive:      true,
		RawInput:      "Spotify todo mês",
	}

	repo := &mockPurchaseRepo{
		findByDescriptionFn: func(_ context.Context, _ string) ([]domain.Purchase, error) {
			return []domain.Purchase{existing}, nil
		},
	}

	uc := newUC(repo, &mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
		return analysis, nil
	}})

	out, err := uc.ExecuteText(context.Background(), TextInput{Text: "cancelei Spotify"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if out.CancelledDescription != "Spotify" {
		t.Errorf("esperava descrição Spotify, got %s", out.CancelledDescription)
	}
}
