package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func TestExecuteText_Success(t *testing.T) {
	uc := newUC(
		successRepo(),
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return singleAnalysis(50.0, "Almoço", "FOOD", 0.95), nil
		}},
	)

	out, err := uc.ExecuteText(context.Background(), TextInput{Text: "gastei 50 no almoço pix"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if out.Amount != 50.0 {
		t.Errorf("amount esperado 50.0, got %v", out.Amount)
	}
	if out.Payment != "Pix" {
		t.Errorf("payment esperado Pix, got %s", out.Payment)
	}
	if out.Category != "Alimentação" {
		t.Errorf("category esperada Alimentação, got %s", out.Category)
	}
	if out.Confidence != 0.95 {
		t.Errorf("confidence esperada 0.95, got %v", out.Confidence)
	}
	if out.Type != "SINGLE" {
		t.Errorf("type esperado SINGLE, got %s", out.Type)
	}
}

func TestExecuteText_AnalyzerError(t *testing.T) {
	uc := newUC(
		successRepo(),
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return nil, errors.New("falha IA")
		}},
	)

	_, err := uc.ExecuteText(context.Background(), TextInput{Text: "qualquer coisa"})
	if err == nil {
		t.Fatal("esperava erro")
	}
}

func TestExecuteText_AmountNil(t *testing.T) {
	uc := newUC(
		successRepo(),
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return &ports.ExpenseAnalysis{Amount: nil, Confidence: 0.4, Type: ports.ExpenseTypeSingle}, nil
		}},
	)

	_, err := uc.ExecuteText(context.Background(), TextInput{Text: "texto"})
	if err == nil {
		t.Fatal("esperava erro de valor nil")
	}
}

func TestExecuteText_RepoSaveError(t *testing.T) {
	repo := &mockPurchaseRepo{
		saveFn: func(_ context.Context, _ *domain.Purchase, _ []domain.Payment) error {
			return errors.New("db down")
		},
	}
	uc := newUC(
		repo,
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return singleAnalysis(30.0, "taxi", "TRANSPORT", 0.9), nil
		}},
	)

	_, err := uc.ExecuteText(context.Background(), TextInput{Text: "taxi debito"})
	if err == nil {
		t.Fatal("esperava erro do repo")
	}
}

func TestExecuteText_DescriptionFallsBackToRawInput(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{Amount: ptr(10.0), Description: nil, Category: ptr("OTHER"), Confidence: 0.8, Type: ports.ExpenseTypeSingle}
	uc := newUC(
		successRepo(),
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}},
	)

	out, err := uc.ExecuteText(context.Background(), TextInput{Text: "dinheiro"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if out.Description != "dinheiro" {
		t.Errorf("description esperada 'dinheiro', got '%s'", out.Description)
	}
}

func TestExecuteText_EmptyDescription_FallsBackToRawInput(t *testing.T) {
	analysis := &ports.ExpenseAnalysis{Amount: ptr(10.0), Description: ptr(""), Category: ptr("OTHER"), Confidence: 0.8, Type: ports.ExpenseTypeSingle}
	uc := newUC(
		successRepo(),
		&mockAnalyzer{analyzeTextFn: func(_ context.Context, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}},
	)

	out, err := uc.ExecuteText(context.Background(), TextInput{Text: "espécie"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if out.Description != "espécie" {
		t.Errorf("description esperada 'espécie', got '%s'", out.Description)
	}
}

func TestExecuteImage_Success(t *testing.T) {
	uc := newUC(
		successRepo(),
		&mockAnalyzer{analyzeImageFn: func(_ context.Context, _ []byte, _ string) (*ports.ExpenseAnalysis, error) {
			return singleAnalysis(120.0, "Supermercado", "SHOPPING", 0.88), nil
		}},
	)

	out, err := uc.ExecuteImage(context.Background(), ImageInput{ImageData: []byte{1, 2, 3}, MimeType: "image/jpeg", Caption: "crédito"})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if out.Amount != 120.0 {
		t.Errorf("amount esperado 120.0, got %v", out.Amount)
	}
	if out.Payment != "Cartão de Crédito" {
		t.Errorf("payment esperado Cartão de Crédito, got %s", out.Payment)
	}
	if out.Category != "Compras" {
		t.Errorf("category esperada Compras, got %s", out.Category)
	}
}

func TestExecuteImage_AnalyzerError(t *testing.T) {
	uc := newUC(
		successRepo(),
		&mockAnalyzer{analyzeImageFn: func(_ context.Context, _ []byte, _ string) (*ports.ExpenseAnalysis, error) {
			return nil, errors.New("falha imagem")
		}},
	)

	_, err := uc.ExecuteImage(context.Background(), ImageInput{ImageData: []byte{}, MimeType: "image/png"})
	if err == nil {
		t.Fatal("esperava erro")
	}
}

func TestExecuteImage_RawInputFormat(t *testing.T) {
	analysis := singleAnalysis(50.0, "", "OTHER", 0.7)
	analysis.Description = nil

	var capturedRawInput string
	repo := &mockPurchaseRepo{
		saveFn: func(_ context.Context, p *domain.Purchase, _ []domain.Payment) error {
			capturedRawInput = p.RawInput
			return nil
		},
	}
	uc := newUC(
		repo,
		&mockAnalyzer{analyzeImageFn: func(_ context.Context, _ []byte, _ string) (*ports.ExpenseAnalysis, error) {
			return analysis, nil
		}},
	)

	_, err := uc.ExecuteImage(context.Background(), ImageInput{ImageData: []byte{1}, MimeType: "image/png", Caption: ""})
	if err != nil {
		t.Fatalf("esperava sucesso, got: %v", err)
	}
	if capturedRawInput != "[imagem: image/png]" {
		t.Errorf("rawInput esperado '[imagem: image/png]', got '%s'", capturedRawInput)
	}
}

func TestInferPaymentMethod(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"paguei via PIX", "PIX"},
		{"no crédito", "CREDIT_CARD"},
		{"cartão de credito", "CREDIT_CARD"},
		{"débito", "DEBIT_CARD"},
		{"paguei no debito", "DEBIT_CARD"},
		{"dinheiro vivo", "CASH"},
		{"pagamento em especie", "CASH"},
		{"pagamento em espécie", "CASH"},
		{"sem info", "OTHER"},
	}
	for _, c := range cases {
		got := inferPaymentMethod(c.input)
		if got != c.expected {
			t.Errorf("inferPaymentMethod(%q) = %q, esperado %q", c.input, got, c.expected)
		}
	}
}

func TestParsePaymentMethod(t *testing.T) {
	cases := []struct {
		input    string
		expected domain.PaymentMethod
		wantErr  bool
	}{
		{"CASH", domain.PaymentMethodCash, false},
		{"CREDIT_CARD", domain.PaymentMethodCreditCard, false},
		{"DEBIT_CARD", domain.PaymentMethodDebitCard, false},
		{"PIX", domain.PaymentMethodPix, false},
		{"OTHER", domain.PaymentMethodOther, false},
		{"", domain.PaymentMethodOther, false},
		{"INVALIDO", "", true},
	}
	for _, c := range cases {
		got, err := parsePaymentMethod(c.input)
		if c.wantErr {
			if err == nil {
				t.Errorf("parsePaymentMethod(%q): esperava erro", c.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePaymentMethod(%q): erro inesperado: %v", c.input, err)
		}
		if got != c.expected {
			t.Errorf("parsePaymentMethod(%q) = %q, esperado %q", c.input, got, c.expected)
		}
	}
}

func TestParseCategory(t *testing.T) {
	cases := []struct {
		input    *string
		expected domain.Category
	}{
		{nil, domain.CategoryOther},
		{ptr("FOOD"), domain.CategoryFood},
		{ptr("TRANSPORT"), domain.CategoryTransport},
		{ptr("HEALTH"), domain.CategoryHealth},
		{ptr("ENTERTAINMENT"), domain.CategoryEntertainment},
		{ptr("SHOPPING"), domain.CategoryShopping},
		{ptr("UNKNOWN"), domain.CategoryOther},
	}
	for _, c := range cases {
		got := parseCategory(c.input)
		if got != c.expected {
			label := "nil"
			if c.input != nil {
				label = *c.input
			}
			t.Errorf("parseCategory(%s) = %q, esperado %q", label, got, c.expected)
		}
	}
}

func TestResolvePaymentMethod(t *testing.T) {
	cases := []struct {
		aiSuggestion *string
		fallback     string
		expected     string
	}{
		{ptr("PIX"), "OTHER", "PIX"},
		{ptr("CREDIT_CARD"), "OTHER", "CREDIT_CARD"},
		{ptr(""), "PIX", "PIX"},
		{nil, "DEBIT_CARD", "DEBIT_CARD"},
	}
	for _, c := range cases {
		got := resolvePaymentMethod(c.aiSuggestion, c.fallback)
		if got != c.expected {
			t.Errorf("resolvePaymentMethod(%v, %q) = %q, esperado %q", c.aiSuggestion, c.fallback, got, c.expected)
		}
	}
}

func TestProcessAnalysis_InvalidPaymentMethod(t *testing.T) {
	uc := newUC(successRepo(), &mockAnalyzer{})
	analysis := &ports.ExpenseAnalysis{Amount: ptr(10.0), Description: ptr("test"), Category: ptr("OTHER"), Confidence: 0.9, Type: ports.ExpenseTypeSingle}

	_, err := uc.processAnalysis(context.Background(), analysis, "INVALIDO", "raw")
	if err == nil {
		t.Fatal("esperava erro de método de pagamento inválido")
	}
	if !errors.Is(err, domain.ErrInvalidPaymentMethod) {
		t.Errorf("esperava ErrInvalidPaymentMethod, got: %v", err)
	}
}

func TestProcessAnalysis_InvalidAmount(t *testing.T) {
	uc := newUC(successRepo(), &mockAnalyzer{})
	analysis := &ports.ExpenseAnalysis{Amount: ptr(0.0), Description: ptr("desc"), Category: ptr("FOOD"), Confidence: 0.9, Type: ports.ExpenseTypeSingle}

	_, err := uc.processAnalysis(context.Background(), analysis, "PIX", "raw")
	if err == nil {
		t.Fatal("esperava erro de despesa inválida")
	}
}
