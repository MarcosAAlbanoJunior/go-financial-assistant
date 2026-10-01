package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func TestProcessQuery_CurrentMonth(t *testing.T) {
	now := time.Now().UTC()
	expectedMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	var capturedMonth time.Time
	repo := &mockPurchaseRepo{
		findPaymentsByMonthFn: func(_ context.Context, month time.Time) ([]ports.PaymentSummary, error) {
			capturedMonth = month
			return []ports.PaymentSummary{
				{Category: "FOOD", Total: 250.00},
				{Category: "TRANSPORT", Total: 80.00},
			}, nil
		},
	}
	analyzer := &mockAnalyzer{
		analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return &ports.ExpenseAnalysis{
				Type:      ports.ExpenseTypeQuery,
				QueryInfo: &ports.QueryInfo{},
			}, nil
		},
	}

	output, err := newUC(repo, analyzer).ExecuteText(context.Background(), TextInput{Text: "quanto gastei esse mês"})

	if err != nil {
		t.Fatalf("esperava sem erro, got: %v", err)
	}
	if output.Type != "QUERY" {
		t.Errorf("tipo esperado QUERY, got %s", output.Type)
	}
	if !capturedMonth.Equal(expectedMonth) {
		t.Errorf("mês esperado %v, got %v", expectedMonth, capturedMonth)
	}
	if output.QueryTotal != 330.00 {
		t.Errorf("total esperado 330.00, got %.2f", output.QueryTotal)
	}
	if len(output.QueryCategories) != 2 {
		t.Errorf("esperava 2 categorias, got %d", len(output.QueryCategories))
	}
	if output.QueryEmpty {
		t.Error("QueryEmpty não deveria ser true")
	}
}

func TestProcessQuery_SpecificMonthYear(t *testing.T) {
	var capturedMonth time.Time
	repo := &mockPurchaseRepo{
		findPaymentsByMonthFn: func(_ context.Context, month time.Time) ([]ports.PaymentSummary, error) {
			capturedMonth = month
			return []ports.PaymentSummary{
				{Category: "SHOPPING", Total: 500.00},
			}, nil
		},
	}
	analyzer := &mockAnalyzer{
		analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return &ports.ExpenseAnalysis{
				Type:      ports.ExpenseTypeQuery,
				QueryInfo: &ports.QueryInfo{Month: 3, Year: 2025},
			}, nil
		},
	}

	output, err := newUC(repo, analyzer).ExecuteText(context.Background(), TextInput{Text: "despesas de março 2025"})

	if err != nil {
		t.Fatalf("esperava sem erro, got: %v", err)
	}
	expectedMonth := time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC)
	if !capturedMonth.Equal(expectedMonth) {
		t.Errorf("mês esperado %v, got %v", expectedMonth, capturedMonth)
	}
	if output.QueryMonth != "Março 2025" {
		t.Errorf("QueryMonth esperado 'Março 2025', got '%s'", output.QueryMonth)
	}
	if output.QueryTotal != 500.00 {
		t.Errorf("total esperado 500.00, got %.2f", output.QueryTotal)
	}
}

func TestProcessQuery_EmptyResult(t *testing.T) {
	repo := &mockPurchaseRepo{
		findPaymentsByMonthFn: func(_ context.Context, _ time.Time) ([]ports.PaymentSummary, error) {
			return nil, nil
		},
	}
	analyzer := &mockAnalyzer{
		analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return &ports.ExpenseAnalysis{
				Type:      ports.ExpenseTypeQuery,
				QueryInfo: &ports.QueryInfo{Month: 1, Year: 2020},
			}, nil
		},
	}

	output, err := newUC(repo, analyzer).ExecuteText(context.Background(), TextInput{Text: "despesas de janeiro 2020"})

	if err != nil {
		t.Fatalf("esperava sem erro, got: %v", err)
	}
	if !output.QueryEmpty {
		t.Error("QueryEmpty deveria ser true para resultado vazio")
	}
	if output.QueryTotal != 0 {
		t.Errorf("total esperado 0, got %.2f", output.QueryTotal)
	}
	if output.QueryMonth != "Janeiro 2020" {
		t.Errorf("QueryMonth esperado 'Janeiro 2020', got '%s'", output.QueryMonth)
	}
}

func TestProcessQuery_RepoError(t *testing.T) {
	repo := &mockPurchaseRepo{
		findPaymentsByMonthFn: func(_ context.Context, _ time.Time) ([]ports.PaymentSummary, error) {
			return nil, errors.New("db error")
		},
	}
	analyzer := &mockAnalyzer{
		analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return &ports.ExpenseAnalysis{Type: ports.ExpenseTypeQuery}, nil
		},
	}

	_, err := newUC(repo, analyzer).ExecuteText(context.Background(), TextInput{Text: "resumo"})

	if err == nil {
		t.Error("esperava erro, got nil")
	}
}

func TestProcessQuery_NoQueryInfo_UsesCurrentMonth(t *testing.T) {
	now := time.Now().UTC()
	expected := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	var capturedMonth time.Time
	repo := &mockPurchaseRepo{
		findPaymentsByMonthFn: func(_ context.Context, month time.Time) ([]ports.PaymentSummary, error) {
			capturedMonth = month
			return nil, nil
		},
	}
	analyzer := &mockAnalyzer{
		analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return &ports.ExpenseAnalysis{Type: ports.ExpenseTypeQuery, QueryInfo: nil}, nil
		},
	}

	newUC(repo, analyzer).ExecuteText(context.Background(), TextInput{Text: "resumo"}) //nolint:errcheck

	if !capturedMonth.Equal(expected) {
		t.Errorf("esperava mês atual %v, got %v", expected, capturedMonth)
	}
}

func TestProcessQuery_WithIncomeAndTransfers(t *testing.T) {
	repo := &mockPurchaseRepo{
		findPaymentsByMonthFn: func(_ context.Context, _ time.Time) ([]ports.PaymentSummary, error) {
			return []ports.PaymentSummary{{Category: "FOOD", Total: 1000.00}}, nil
		},
		findIncomeTotalByMonthFn: func(_ context.Context, _ time.Time) (float64, error) {
			return 6000.00, nil
		},
		findTransferNetByMonthFn: func(_ context.Context, _ time.Time) (float64, float64, error) {
			return 3000.00, 500.00, nil // applied=3000, redeemed=500
		},
	}
	analyzer := &mockAnalyzer{
		analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return &ports.ExpenseAnalysis{Type: ports.ExpenseTypeQuery, QueryInfo: &ports.QueryInfo{Month: 3, Year: 2026}}, nil
		},
	}

	output, err := newUC(repo, analyzer).ExecuteText(context.Background(), TextInput{Text: "resumo março 2026"})

	if err != nil {
		t.Fatalf("esperava sem erro, got: %v", err)
	}
	if output.QueryTotal != 1000.00 {
		t.Errorf("QueryTotal: esperava 1000.00, got %.2f", output.QueryTotal)
	}
	if output.QueryIncome != 6000.00 {
		t.Errorf("QueryIncome: esperava 6000.00, got %.2f", output.QueryIncome)
	}
	if output.QueryBalance != 5000.00 {
		t.Errorf("QueryBalance: esperava 5000.00, got %.2f", output.QueryBalance)
	}
	if output.QueryApplied != 3000.00 {
		t.Errorf("QueryApplied: esperava 3000.00, got %.2f", output.QueryApplied)
	}
	if output.QueryRedeemed != 500.00 {
		t.Errorf("QueryRedeemed: esperava 500.00, got %.2f", output.QueryRedeemed)
	}
	if output.QueryNetInvested != 2500.00 {
		t.Errorf("QueryNetInvested: esperava 2500.00, got %.2f", output.QueryNetInvested)
	}
	// em conta = resultado(5000) - líquido investido(2500) = 2500
	if output.QueryInAccount != 2500.00 {
		t.Errorf("QueryInAccount: esperava 2500.00, got %.2f", output.QueryInAccount)
	}
}

func TestProcessQuery_EmptyWithTransfers_NotEmpty(t *testing.T) {
	repo := &mockPurchaseRepo{
		findPaymentsByMonthFn: func(_ context.Context, _ time.Time) ([]ports.PaymentSummary, error) {
			return nil, nil
		},
		findTransferNetByMonthFn: func(_ context.Context, _ time.Time) (float64, float64, error) {
			return 1000.00, 0, nil
		},
	}
	analyzer := &mockAnalyzer{
		analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return &ports.ExpenseAnalysis{Type: ports.ExpenseTypeQuery, QueryInfo: &ports.QueryInfo{Month: 1, Year: 2026}}, nil
		},
	}

	output, err := newUC(repo, analyzer).ExecuteText(context.Background(), TextInput{Text: "resumo"})

	if err != nil {
		t.Fatalf("esperava sem erro, got: %v", err)
	}
	if output.QueryEmpty {
		t.Error("QueryEmpty não deveria ser true quando há transferências")
	}
}

func TestResolveQueryMonth_InvalidMonth(t *testing.T) {
	info := &ports.QueryInfo{Month: 13, Year: 2025}
	now := time.Now().UTC()
	result := resolveQueryMonth(info)

	if result.Month() != now.Month() {
		t.Errorf("mês inválido deveria usar mês atual, got %d", result.Month())
	}
	if result.Year() != 2025 {
		t.Errorf("ano deveria ser 2025, got %d", result.Year())
	}
}

func TestFormatMonthPT(t *testing.T) {
	cases := []struct {
		month    time.Month
		year     int
		expected string
	}{
		{time.January, 2025, "Janeiro 2025"},
		{time.March, 2025, "Março 2025"},
		{time.December, 2024, "Dezembro 2024"},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			got := formatMonthPT(time.Date(tc.year, tc.month, 1, 0, 0, 0, 0, time.UTC))
			if got != tc.expected {
				t.Errorf("esperava '%s', got '%s'", tc.expected, got)
			}
		})
	}
}
