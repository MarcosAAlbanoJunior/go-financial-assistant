package ledger

import (
	"context"
	"log/slog"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

type mockAnalyzer struct {
	analyzeTextFn     func(ctx context.Context, text string) (*ports.ExpenseAnalysis, error)
	analyzeImageFn    func(ctx context.Context, imageData []byte, mimeType string) (*ports.ExpenseAnalysis, error)
	analyzeDocumentFn func(ctx context.Context, data []byte, mimeType string) (*ports.StatementAnalysis, error)
}

func (m *mockAnalyzer) AnalyzeText(ctx context.Context, text string) (*ports.ExpenseAnalysis, error) {
	return m.analyzeTextFn(ctx, text)
}

func (m *mockAnalyzer) AnalyzeImage(ctx context.Context, imageData []byte, mimeType string) (*ports.ExpenseAnalysis, error) {
	return m.analyzeImageFn(ctx, imageData, mimeType)
}

func (m *mockAnalyzer) AnalyzeDocument(ctx context.Context, data []byte, mimeType string) (*ports.StatementAnalysis, error) {
	if m.analyzeDocumentFn != nil {
		return m.analyzeDocumentFn(ctx, data, mimeType)
	}
	return &ports.StatementAnalysis{}, nil
}

// SavePendingTransaction não é método do AIAnalyzer — pertence ao use case AnalyzeExpense.

type mockLedger struct {
	saveFn                         func(ctx context.Context, purchase *domain.Purchase, payments []domain.Payment) error
	findActiveRecurringFn          func(ctx context.Context) ([]domain.Purchase, error)
	findByDescriptionFn            func(ctx context.Context, description string) ([]domain.Purchase, error)
	updateFn                       func(ctx context.Context, purchase *domain.Purchase) error
	savePaymentFn                  func(ctx context.Context, payment *domain.Payment) error
	hasPaymentForMonthFn           func(ctx context.Context, purchaseID uuid.UUID, month time.Time) (bool, error)
	findPaymentsByMonthFn          func(ctx context.Context, month time.Time) ([]ports.PaymentSummary, error)
	findPaymentDetailsByMonthFn    func(ctx context.Context, month time.Time) ([]ports.PaymentDetail, error)
	findIncomeTotalByMonthFn       func(ctx context.Context, month time.Time) (float64, error)
	findTransferNetByMonthFn       func(ctx context.Context, month time.Time) (float64, float64, error)
	existsPaymentByDateAndAmountFn func(ctx context.Context, date time.Time, amount float64) (bool, error)
}

func (m *mockLedger) Save(ctx context.Context, purchase *domain.Purchase, payments []domain.Payment) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, purchase, payments)
	}
	return nil
}

func (m *mockLedger) FindActiveRecurring(ctx context.Context) ([]domain.Purchase, error) {
	if m.findActiveRecurringFn != nil {
		return m.findActiveRecurringFn(ctx)
	}
	return nil, nil
}

func (m *mockLedger) FindByDescription(ctx context.Context, description string) ([]domain.Purchase, error) {
	if m.findByDescriptionFn != nil {
		return m.findByDescriptionFn(ctx, description)
	}
	return nil, nil
}

func (m *mockLedger) Update(ctx context.Context, purchase *domain.Purchase) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, purchase)
	}
	return nil
}

func (m *mockLedger) SavePayment(ctx context.Context, payment *domain.Payment) error {
	if m.savePaymentFn != nil {
		return m.savePaymentFn(ctx, payment)
	}
	return nil
}

func (m *mockLedger) HasPaymentForMonth(ctx context.Context, purchaseID uuid.UUID, month time.Time) (bool, error) {
	if m.hasPaymentForMonthFn != nil {
		return m.hasPaymentForMonthFn(ctx, purchaseID, month)
	}
	return false, nil
}

func (m *mockLedger) FindPaymentsByMonth(ctx context.Context, month time.Time) ([]ports.PaymentSummary, error) {
	if m.findPaymentsByMonthFn != nil {
		return m.findPaymentsByMonthFn(ctx, month)
	}
	return nil, nil
}

func (m *mockLedger) FindPaymentDetailsByMonth(ctx context.Context, month time.Time) ([]ports.PaymentDetail, error) {
	if m.findPaymentDetailsByMonthFn != nil {
		return m.findPaymentDetailsByMonthFn(ctx, month)
	}
	return nil, nil
}

func (m *mockLedger) FindIncomeTotalByMonth(ctx context.Context, month time.Time) (float64, error) {
	if m.findIncomeTotalByMonthFn != nil {
		return m.findIncomeTotalByMonthFn(ctx, month)
	}
	return 0, nil
}

func (m *mockLedger) FindTransferNetByMonth(ctx context.Context, month time.Time) (float64, float64, error) {
	if m.findTransferNetByMonthFn != nil {
		return m.findTransferNetByMonthFn(ctx, month)
	}
	return 0, 0, nil
}

func (m *mockLedger) ExistsPaymentByDateAndAmount(ctx context.Context, date time.Time, amount float64) (bool, error) {
	if m.existsPaymentByDateAndAmountFn != nil {
		return m.existsPaymentByDateAndAmountFn(ctx, date, amount)
	}
	return false, nil
}

func ptr[T any](v T) *T { return &v }

func successRepo() *mockLedger {
	return &mockLedger{}
}

func newUC(repo *mockLedger, analyzer *mockAnalyzer) *AnalyzeExpense {
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
