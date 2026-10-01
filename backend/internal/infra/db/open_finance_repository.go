package db

import (
	"context"
	"fmt"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func (r *PostgresPurchaseRepository) ExistsExternalID(ctx context.Context, externalID string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM payments WHERE external_id = $1)`, externalID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("erro ao verificar transação externa: %w", err)
	}
	return exists, nil
}

func (r *PostgresPurchaseRepository) SaveExternal(ctx context.Context, purchase *domain.Purchase, payment *domain.Payment) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := insertPurchase(ctx, tx, purchase); err != nil {
		return err
	}

	query := `
		INSERT INTO payments
			(id, purchase_id, amount, status, due_date, paid_at, external_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	if _, err := tx.Exec(ctx, query,
		payment.ID, payment.PurchaseID, payment.Amount, payment.Status,
		payment.DueDate, payment.PaidAt, payment.ExternalID, payment.CreatedAt,
	); err != nil {
		return fmt.Errorf("erro ao salvar pagamento externo: %w", err)
	}

	return tx.Commit(ctx)
}

// ReconcileExternal procura um lançamento manual (sem external_id) equivalente à transação
// do banco e o vincula, em vez de criar um segundo lançamento. Casa por tipo, valor e data:
//   - SINGLE: data do lançamento até 3 dias de diferença;
//   - RECURRING: mesmo mês (a recorrência é gerada em dia fixo, o banco pode atrasar).
//
// Parcelados ficam de fora: a data de cada parcela no cartão não é previsível.
func (r *PostgresPurchaseRepository) ReconcileExternal(ctx context.Context, tx ports.ExternalTransaction) (bool, error) {
	query := `
		UPDATE payments SET external_id = $1, status = 'PAID', paid_at = COALESCE(paid_at, $5::timestamptz)
		WHERE id = (
			SELECT pay.id
			FROM payments pay
			JOIN purchases p ON p.id = pay.purchase_id
			WHERE pay.external_id IS NULL
			  AND pay.status != 'CANCELLED'
			  AND p.kind = $2
			  AND ABS(pay.amount - $3) < 0.005
			  AND (
			        (p.type = 'SINGLE'
			         AND ABS(COALESCE(pay.due_date, pay.created_at::date) - $4::date) <= 3)
			     OR (p.type = 'RECURRING'
			         AND DATE_TRUNC('month', COALESCE(pay.due_date, pay.reference_month)) = DATE_TRUNC('month', $4::date))
			  )
			ORDER BY pay.created_at
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
	`
	tag, err := r.db.Pool.Exec(ctx, query, tx.ID, tx.Kind, tx.Amount, tx.Date, tx.Date)
	if err != nil {
		return false, fmt.Errorf("erro ao conciliar transação externa: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
