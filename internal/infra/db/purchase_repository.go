package db

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type purchaseModel struct {
	ID                 uuid.UUID  `db:"id"`
	Description        *string    `db:"description"`
	Category           string     `db:"category"`
	PaymentMethod      string     `db:"payment_method"`
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
			(id, description, category, payment_method, type, total_amount,
			 installment_count, installment_amount, day_of_month, is_active, raw_input, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	if _, err := tx.Exec(ctx, purchaseQuery,
		purchase.ID, purchase.Description, purchase.Category, purchase.PaymentMethod,
		purchase.Type, purchase.TotalAmount, purchase.InstallmentCount,
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

func (r *PostgresPurchaseRepository) FindActiveRecurring(ctx context.Context) ([]domain.Purchase, error) {
	query := `
		SELECT id, description, category, payment_method, type, total_amount,
		       installment_count, installment_amount, day_of_month, is_active,
		       cancelled_at, cancellation_reason, raw_input, created_at
		FROM purchases
		WHERE type = 'RECURRING' AND is_active = TRUE
		ORDER BY description
	`
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar despesas recorrentes ativas: %w", err)
	}

	models, err := pgx.CollectRows(rows, pgx.RowToStructByName[purchaseModel])
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear compras: %w", err)
	}

	result := make([]domain.Purchase, len(models))
	for i, m := range models {
		result[i] = m.toDomain()
	}
	return result, nil
}

func (r *PostgresPurchaseRepository) FindByDescription(ctx context.Context, description string) ([]domain.Purchase, error) {
	query := `
		SELECT id, description, category, payment_method, type, total_amount,
		       installment_count, installment_amount, day_of_month, is_active,
		       cancelled_at, cancellation_reason, raw_input, created_at
		FROM purchases
		WHERE type = 'RECURRING' AND is_active = TRUE AND description ILIKE $1
		ORDER BY description
	`
	rows, err := r.db.Pool.Query(ctx, query, "%"+description+"%")
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar compras por descrição: %w", err)
	}

	models, err := pgx.CollectRows(rows, pgx.RowToStructByName[purchaseModel])
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear compras: %w", err)
	}

	result := make([]domain.Purchase, len(models))
	for i, m := range models {
		result[i] = m.toDomain()
	}
	return result, nil
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

func (r *PostgresPurchaseRepository) HasPaymentForMonth(ctx context.Context, purchaseID uuid.UUID, month time.Time) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM payments WHERE purchase_id = $1 AND reference_month = $2)`
	var exists bool
	if err := r.db.Pool.QueryRow(ctx, query, purchaseID, month).Scan(&exists); err != nil {
		return false, fmt.Errorf("erro ao verificar pagamento do mês: %w", err)
	}
	return exists, nil
}

func (r *PostgresPurchaseRepository) FindPaymentsByMonth(ctx context.Context, month time.Time) ([]ports.PaymentSummary, error) {
	query := `
		SELECT p.category, SUM(pay.amount) AS total
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE DATE_TRUNC('month', COALESCE(pay.due_date, pay.reference_month, pay.created_at)) = DATE_TRUNC('month', $1::timestamptz)
		  AND pay.status != 'CANCELLED'
		GROUP BY p.category
		ORDER BY total DESC
	`
	rows, err := r.db.Pool.Query(ctx, query, month)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar despesas do mês: %w", err)
	}
	defer rows.Close()

	var result []ports.PaymentSummary
	for rows.Next() {
		var s ports.PaymentSummary
		if err := rows.Scan(&s.Category, &s.Total); err != nil {
			return nil, fmt.Errorf("erro ao escanear resumo: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
