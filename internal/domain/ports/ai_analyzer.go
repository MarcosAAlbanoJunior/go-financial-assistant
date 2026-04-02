package ports

import (
	"context"
	"time"
)

type ExpenseType string

const (
	ExpenseTypeSingle          ExpenseType = "SINGLE"
	ExpenseTypeInstallment     ExpenseType = "INSTALLMENT"
	ExpenseTypeRecurring       ExpenseType = "RECURRING"
	ExpenseTypeCancelRecurring ExpenseType = "CANCEL_RECURRING"
	ExpenseTypeQuery           ExpenseType = "QUERY"
	ExpenseTypeExportCSV       ExpenseType = "EXPORT_CSV"
	ExpenseTypeIncome          ExpenseType = "INCOME"
	ExpenseTypeIncomeRecurring ExpenseType = "INCOME_RECURRING"
)

type AIAnalyzer interface {
	AnalyzeText(ctx context.Context, text string) (*ExpenseAnalysis, error)
	AnalyzeImage(ctx context.Context, imageData []byte, mimeType string) (*ExpenseAnalysis, error)
	AnalyzeDocument(ctx context.Context, data []byte, mimeType string) (*StatementAnalysis, error)
}

type StatementAnalysis struct {
	Transactions []StatementTransaction
}

type StatementTransaction struct {
	Date           time.Time
	RawDescription string
	Description    string
	Amount         float64
	Category       string
	PaymentMethod  string
	Kind           string // "EXPENSE" ou "INCOME"
}

type ExpenseAnalysis struct {
	Amount        *float64
	Description   *string
	Category      *string
	PaymentMethod *string
	Confidence    float64
	RawResponse   string
	Type          ExpenseType
	Installments  *InstallmentInfo
	RecurringInfo *RecurringInfo
	CancelInfo    *CancelInfo
	QueryInfo     *QueryInfo
	ExportInfo    *QueryInfo
}

type InstallmentInfo struct {
	Total                int
	AmountPerInstallment float64
}

type RecurringInfo struct {
	DayOfMonth int
}

type CancelInfo struct {
	Description string
}

type QueryInfo struct {
	Month int
	Year  int
}
