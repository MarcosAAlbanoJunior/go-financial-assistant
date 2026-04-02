package db

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

type purchaseModel struct {
	ID                 uuid.UUID  `db:"id"`
	Description        *string    `db:"description"`
	Category           string     `db:"category"`
	PaymentMethod      string     `db:"payment_method"`
	Kind               string     `db:"kind"`
	Type               string     `db:"type"`
	TotalAmount        float64    `db:"total_amount"`
	InstallmentCount   *int       `db:"installment_count"`
	InstallmentAmount  *float64   `db:"installment_amount"`
	DayOfMonth         *int       `db:"day_of_month"`
	IsActive           bool       `db:"is_active"`
	CancelledAt        *time.Time `db:"cancelled_at"`
	CancellationReason *string    `db:"cancellation_reason"`
	RawInput           string     `db:"raw_input"`
	CreatedAt          time.Time  `db:"created_at"`
}

func (m purchaseModel) toDomain() domain.Purchase {
	return domain.Purchase{
		ID:                 m.ID,
		Description:        m.Description,
		Category:           domain.Category(m.Category),
		PaymentMethod:      domain.PaymentMethod(m.PaymentMethod),
		Kind:               domain.PurchaseKind(m.Kind),
		Type:               domain.PurchaseType(m.Type),
		TotalAmount:        m.TotalAmount,
		InstallmentCount:   m.InstallmentCount,
		InstallmentAmount:  m.InstallmentAmount,
		DayOfMonth:         m.DayOfMonth,
		IsActive:           m.IsActive,
		CancelledAt:        m.CancelledAt,
		CancellationReason: m.CancellationReason,
		RawInput:           m.RawInput,
		CreatedAt:          m.CreatedAt,
	}
}

type paymentModel struct {
	ID                uuid.UUID  `db:"id"`
	PurchaseID        uuid.UUID  `db:"purchase_id"`
	Amount            float64    `db:"amount"`
	Status            string     `db:"status"`
	InstallmentNumber *int       `db:"installment_number"`
	DueDate           *time.Time `db:"due_date"`
	ReferenceMonth    *time.Time `db:"reference_month"`
	PaidAt            *time.Time `db:"paid_at"`
	CreatedAt         time.Time  `db:"created_at"`
}

type PostgresPurchaseRepository struct {
	db *DB
}

func NewPurchaseRepository(db *DB) ports.PurchaseRepository {
	return &PostgresPurchaseRepository{db: db}
}

func (r *PostgresPurchaseRepository) Save(ctx context.Context, purchase *domain.Purchase, payments []domain.Payment) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	purchaseQuery := `
		INSERT INTO purchases
			(id, description, category, payment_method, kind, type, total_amount,
			 installment_count, installment_amount, day_of_month, is_active, raw_input, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	if _, err := tx.Exec(ctx, purchaseQuery,
		purchase.ID, purchase.Description, purchase.Category, purchase.PaymentMethod,
		purchase.Kind, purchase.Type, purchase.TotalAmount, purchase.InstallmentCount,
		purchase.InstallmentAmount, purchase.DayOfMonth, purchase.IsActive,
		purchase.RawInput, purchase.CreatedAt,
	); err != nil {
		return fmt.Errorf("erro ao salvar compra: %w", err)
	}

	paymentQuery := `
		INSERT INTO payments
			(id, purchase_id, amount, status, installment_number, due_date, reference_month, paid_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	for _, p := range payments {
		if _, err := tx.Exec(ctx, paymentQuery,
			p.ID, p.PurchaseID, p.Amount, p.Status,
			p.InstallmentNumber, p.DueDate, p.ReferenceMonth, p.PaidAt, p.CreatedAt,
		); err != nil {
			return fmt.Errorf("erro ao salvar pagamento: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *PostgresPurchaseRepository) Update(ctx context.Context, purchase *domain.Purchase) error {
	query := `
		UPDATE purchases
		SET is_active = $2, cancelled_at = $3, cancellation_reason = $4
		WHERE id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query,
		purchase.ID, purchase.IsActive, purchase.CancelledAt, purchase.CancellationReason,
	)
	if err != nil {
		return fmt.Errorf("erro ao atualizar compra: %w", err)
	}
	return nil
}

func (r *PostgresPurchaseRepository) FindIncomeTotalByMonth(ctx context.Context, month time.Time) (float64, error) {
	query := `
		SELECT COALESCE(SUM(pay.amount), 0)
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE DATE_TRUNC('month', COALESCE(pay.due_date, pay.reference_month, pay.created_at)) = DATE_TRUNC('month', $1::timestamptz)
		  AND p.kind = 'INCOME'
		  AND pay.status != 'CANCELLED'
	`
	var total float64
	if err := r.db.Pool.QueryRow(ctx, query, month).Scan(&total); err != nil {
		return 0, fmt.Errorf("erro ao consultar entradas do mês: %w", err)
	}
	return total, nil
}

func (r *PostgresPurchaseRepository) SavePayment(ctx context.Context, payment *domain.Payment) error {
	query := `
		INSERT INTO payments
			(id, purchase_id, amount, status, installment_number, due_date, reference_month, paid_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		payment.ID, payment.PurchaseID, payment.Amount, payment.Status,
		payment.InstallmentNumber, payment.DueDate, payment.ReferenceMonth,
		payment.PaidAt, payment.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("erro ao salvar pagamento: %w", err)
	}
	return nil
}
