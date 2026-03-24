package usecase

import (
	"context"
	"log/slog"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

// ---- mock AIAnalyzer ----

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

// ---- mock ExpenseRepository ----

type mockRepo struct {
	saveFn   func(ctx context.Context, expense *domain.Expense) error
	findByID func(ctx context.Context, id uuid.UUID) (*domain.Expense, error)
	findAll  func(ctx context.Context, filter ports.ExpenseFilter) ([]domain.Expense, error)
	deleteFn func(ctx context.Context, id uuid.UUID) error
}

func (m *mockRepo) Save(ctx context.Context, expense *domain.Expense) error {
	return m.saveFn(ctx, expense)
}

func (m *mockRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Expense, error) {
	return m.findByID(ctx, id)
}

func (m *mockRepo) FindAll(ctx context.Context, filter ports.ExpenseFilter) ([]domain.Expense, error) {
	return m.findAll(ctx, filter)
}

func (m *mockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.deleteFn(ctx, id)
}

// ---- mock InstallmentRepository ----

type mockInstallRepo struct {
	savePurchaseFn               func(ctx context.Context, purchase *domain.InstallmentPurchase, installments []domain.Installment) error
	findPurchaseByIDFn           func(ctx context.Context, id uuid.UUID) (*domain.InstallmentPurchase, error)
	findInstallmentsByPurchaseID func(ctx context.Context, purchaseID uuid.UUID) ([]domain.Installment, error)
	findPendingInstallmentsFn    func(ctx context.Context) ([]domain.Installment, error)
}

func (m *mockInstallRepo) SavePurchase(ctx context.Context, purchase *domain.InstallmentPurchase, installments []domain.Installment) error {
	if m.savePurchaseFn != nil {
		return m.savePurchaseFn(ctx, purchase, installments)
	}
	return nil
}

func (m *mockInstallRepo) FindPurchaseByID(ctx context.Context, id uuid.UUID) (*domain.InstallmentPurchase, error) {
	if m.findPurchaseByIDFn != nil {
		return m.findPurchaseByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockInstallRepo) FindInstallmentsByPurchaseID(ctx context.Context, purchaseID uuid.UUID) ([]domain.Installment, error) {
	if m.findInstallmentsByPurchaseID != nil {
		return m.findInstallmentsByPurchaseID(ctx, purchaseID)
	}
	return nil, nil
}

func (m *mockInstallRepo) FindPendingInstallments(ctx context.Context) ([]domain.Installment, error) {
	if m.findPendingInstallmentsFn != nil {
		return m.findPendingInstallmentsFn(ctx)
	}
	return nil, nil
}

// ---- mock RecurringExpenseRepository ----

type mockRecurringRepo struct {
	saveFn              func(ctx context.Context, expense *domain.RecurringExpense) error
	findByIDFn          func(ctx context.Context, id uuid.UUID) (*domain.RecurringExpense, error)
	findActiveFn        func(ctx context.Context) ([]domain.RecurringExpense, error)
	updateFn            func(ctx context.Context, expense *domain.RecurringExpense) error
	findByDescriptionFn func(ctx context.Context, description string) ([]domain.RecurringExpense, error)
}

func (m *mockRecurringRepo) Save(ctx context.Context, expense *domain.RecurringExpense) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, expense)
	}
	return nil
}

func (m *mockRecurringRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.RecurringExpense, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockRecurringRepo) FindActive(ctx context.Context) ([]domain.RecurringExpense, error) {
	if m.findActiveFn != nil {
		return m.findActiveFn(ctx)
	}
	return nil, nil
}

func (m *mockRecurringRepo) Update(ctx context.Context, expense *domain.RecurringExpense) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, expense)
	}
	return nil
}

func (m *mockRecurringRepo) FindByDescription(ctx context.Context, description string) ([]domain.RecurringExpense, error) {
	if m.findByDescriptionFn != nil {
		return m.findByDescriptionFn(ctx, description)
	}
	return nil, nil
}

// ---- shared helpers ----

func ptr[T any](v T) *T { return &v }

func successRepo() *mockRepo {
	return &mockRepo{
		saveFn: func(_ context.Context, _ *domain.Expense) error { return nil },
	}
}

func noopInstallRepo() *mockInstallRepo   { return &mockInstallRepo{} }
func noopRecurringRepo() *mockRecurringRepo { return &mockRecurringRepo{} }

func newUC(repo *mockRepo, analyzer *mockAnalyzer) *AnalyzeExpense {
	return NewAnalyzeExpense(repo, noopInstallRepo(), noopRecurringRepo(), analyzer, slog.Default())
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
