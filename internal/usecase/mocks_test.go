package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

type mockAnalyzer struct {
	analyzeTextFn  func(ctx context.Context, text string) (*ports.ExpenseAnalysis, error)
	analyzeImageFn func(ctx context.Context, imageData []byte, mimeType string) (*ports.ExpenseAnalysis, error)
}

func (m *mockAnalyzer) AnalyzeText(ctx context.Context, text string) (*ports.ExpenseAnalysis, error) {
	return m.analyzeTextFn(ctx, text)
}

func (m *mockAnalyzer) AnalyzeImage(ctx context.Context, imageData []byte, mimeType string) (*ports.ExpenseAnalysis, error) {
	return m.analyzeImageFn(ctx, imageData, mimeType)
}

type mockPurchaseRepo struct {
	saveFn                  func(ctx context.Context, purchase *domain.Purchase, payments []domain.Payment) error
	findActiveRecurringFn   func(ctx context.Context) ([]domain.Purchase, error)
	findByDescriptionFn     func(ctx context.Context, description string) ([]domain.Purchase, error)
	updateFn                func(ctx context.Context, purchase *domain.Purchase) error
	savePaymentFn           func(ctx context.Context, payment *domain.Payment) error
	hasPaymentForMonthFn    func(ctx context.Context, purchaseID uuid.UUID, month time.Time) (bool, error)
	findPaymentsByMonthFn   func(ctx context.Context, month time.Time) ([]ports.CategorySummary, error)
}

func (m *mockPurchaseRepo) Save(ctx context.Context, purchase *domain.Purchase, payments []domain.Payment) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, purchase, payments)
	}
	return nil
}

func (m *mockPurchaseRepo) FindActiveRecurring(ctx context.Context) ([]domain.Purchase, error) {
	if m.findActiveRecurringFn != nil {
		return m.findActiveRecurringFn(ctx)
	}
	return nil, nil
}

func (m *mockPurchaseRepo) FindByDescription(ctx context.Context, description string) ([]domain.Purchase, error) {
	if m.findByDescriptionFn != nil {
		return m.findByDescriptionFn(ctx, description)
	}
	return nil, nil
}

func (m *mockPurchaseRepo) Update(ctx context.Context, purchase *domain.Purchase) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, purchase)
	}
	return nil
}

func (m *mockPurchaseRepo) SavePayment(ctx context.Context, payment *domain.Payment) error {
	if m.savePaymentFn != nil {
		return m.savePaymentFn(ctx, payment)
	}
	return nil
}

func (m *mockPurchaseRepo) HasPaymentForMonth(ctx context.Context, purchaseID uuid.UUID, month time.Time) (bool, error) {
	if m.hasPaymentForMonthFn != nil {
		return m.hasPaymentForMonthFn(ctx, purchaseID, month)
	}
	return false, nil
}

func (m *mockPurchaseRepo) FindPaymentsByMonth(ctx context.Context, month time.Time) ([]ports.CategorySummary, error) {
	if m.findPaymentsByMonthFn != nil {
		return m.findPaymentsByMonthFn(ctx, month)
	}
	return nil, nil
}

func ptr[T any](v T) *T { return &v }

func successRepo() *mockPurchaseRepo {
	return &mockPurchaseRepo{}
}

func newUC(repo *mockPurchaseRepo, analyzer *mockAnalyzer) *AnalyzeExpense {
	return NewAnalyzeExpense(repo, analyzer, slog.Default())
}

func singleAnalysis(amount float64, desc, category string, conf float64) *ports.ExpenseAnalysis {
	return &ports.ExpenseAnalysis{
		Amount:      ptr(amount),
		Description: ptr(desc),
		Category:    ptr(category),
		Confidence:  conf,
		Type:        ports.ExpenseTypeSingle,
	}
}
