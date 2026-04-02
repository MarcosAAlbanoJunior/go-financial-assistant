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

type paymentDetailModel struct {
	Description       *string    `db:"description"`
	Category          string     `db:"category"`
	PaymentMethod     string     `db:"payment_method"`
	Amount            float64    `db:"amount"`
	Status            string     `db:"status"`
	PurchaseType      string     `db:"purchase_type"`
	PurchaseKind      string     `db:"purchase_kind"`
	InstallmentNumber *int       `db:"installment_number"`
	DueDate           *time.Time `db:"due_date"`
	ReferenceMonth    *time.Time `db:"reference_month"`
	CreatedAt         time.Time  `db:"created_at"`
}

func (m paymentDetailModel) toPort() ports.PaymentDetail {
	return ports.PaymentDetail{
		Description:       m.Description,
		Category:          m.Category,
		PaymentMethod:     m.PaymentMethod,
		Amount:            m.Amount,
		Status:            m.Status,
		PurchaseType:      m.PurchaseType,
		PurchaseKind:      m.PurchaseKind,
		InstallmentNumber: m.InstallmentNumber,
		DueDate:           m.DueDate,
		ReferenceMonth:    m.ReferenceMonth,
		CreatedAt:         m.CreatedAt,
	}
}

func (r *PostgresPurchaseRepository) ExistsPaymentByDateAndAmount(ctx context.Context, date time.Time, amount float64) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM payments
			WHERE ABS(amount - $2) < 0.001
			  AND DATE_TRUNC('day', COALESCE(due_date, paid_at, created_at)) = DATE_TRUNC('day', $1::timestamptz)
			  AND status != 'CANCELLED'
		)
	`
	var exists bool
	if err := r.db.Pool.QueryRow(ctx, query, date, amount).Scan(&exists); err != nil {
		return false, fmt.Errorf("erro ao verificar duplicata: %w", err)
	}
	return exists, nil
}

func (r *PostgresPurchaseRepository) HasPaymentForMonth(ctx context.Context, purchaseID uuid.UUID, month time.Time) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM payments WHERE purchase_id = $1 AND reference_month = $2)`
	var exists bool
	if err := r.db.Pool.QueryRow(ctx, query, purchaseID, month).Scan(&exists); err != nil {
		return false, fmt.Errorf("erro ao verificar pagamento do mês: %w", err)
	}
	return exists, nil
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

func (r *PostgresPurchaseRepository) FindPaymentsByMonth(ctx context.Context, month time.Time) ([]ports.PaymentSummary, error) {
	query := `
		SELECT p.category, SUM(pay.amount) AS total
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE DATE_TRUNC('month', COALESCE(pay.due_date, pay.reference_month, pay.created_at)) = DATE_TRUNC('month', $1::timestamptz)
		  AND pay.status != 'CANCELLED'
		  AND p.kind = 'EXPENSE'
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

func (r *PostgresPurchaseRepository) FindPaymentDetailsByMonth(ctx context.Context, month time.Time) ([]ports.PaymentDetail, error) {
	query := `
		SELECT
		    p.description,
		    p.category,
		    p.payment_method,
		    pay.amount,
		    pay.status,
		    p.type AS purchase_type,
		    p.kind AS purchase_kind,
		    pay.installment_number,
		    pay.due_date,
		    pay.reference_month,
		    pay.created_at
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE DATE_TRUNC('month', COALESCE(pay.due_date, pay.reference_month, pay.created_at)) = DATE_TRUNC('month', $1::timestamptz)
		  AND pay.status != 'CANCELLED'
		ORDER BY COALESCE(pay.due_date, pay.reference_month, pay.created_at)
	`
	rows, err := r.db.Pool.Query(ctx, query, month)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar detalhes do mês: %w", err)
	}

	models, err := pgx.CollectRows(rows, pgx.RowToStructByName[paymentDetailModel])
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear detalhes: %w", err)
	}

	result := make([]ports.PaymentDetail, len(models))
	for i, m := range models {
		result[i] = m.toPort()
	}
	return result, nil
}

//todo colocar saldo tambem de entradas.