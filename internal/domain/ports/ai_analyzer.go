package ports

import "context"

type ExpenseType string

const (
	ExpenseTypeSingle          ExpenseType = "SINGLE"
	ExpenseTypeInstallment     ExpenseType = "INSTALLMENT"
	ExpenseTypeRecurring       ExpenseType = "RECURRING"
	ExpenseTypeCancelRecurring ExpenseType = "CANCEL_RECURRING"
)

type AIAnalyzer interface {
	AnalyzeText(ctx context.Context, text string) (*ExpenseAnalysis, error)

	AnalyzeImage(ctx context.Context, imageData []byte, mimeType string) (*ExpenseAnalysis, error)
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
