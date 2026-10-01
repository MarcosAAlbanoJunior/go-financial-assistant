package db

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

func (r *PostgresPurchaseRepository) UpsertAccount(ctx context.Context, a ports.ExternalAccount) (uuid.UUID, error) {
	query := `
		INSERT INTO accounts (external_id, item_id, type, name, last4, balance, credit_limit, available_credit_limit, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (external_id) DO UPDATE SET
			item_id = EXCLUDED.item_id, type = EXCLUDED.type, name = EXCLUDED.name, last4 = EXCLUDED.last4,
			balance = EXCLUDED.balance, credit_limit = EXCLUDED.credit_limit,
			available_credit_limit = EXCLUDED.available_credit_limit, updated_at = NOW()
		RETURNING id
	`
	var id uuid.UUID
	if err := r.db.Pool.QueryRow(ctx, query,
		a.ID, a.ItemID, a.Type, a.Name, a.Last4, a.Balance, a.CreditLimit, a.AvailableCreditLimit,
	).Scan(&id); err != nil {
		return uuid.Nil, fmt.Errorf("erro ao salvar conta: %w", err)
	}
	return id, nil
}

func (r *PostgresPurchaseRepository) LinkExternalAccount(ctx context.Context, externalID string, accountID uuid.UUID) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx,
		`UPDATE payments SET account_id = COALESCE(account_id, $2) WHERE external_id = $1`, externalID, accountID)
	if err != nil {
		return false, fmt.Errorf("erro ao verificar transação externa: %w", err)
	}
	return tag.RowsAffected() > 0, nil
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
			(id, purchase_id, amount, status, due_date, paid_at, external_id, account_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if _, err := tx.Exec(ctx, query,
		payment.ID, payment.PurchaseID, payment.Amount, payment.Status,
		payment.DueDate, payment.PaidAt, payment.ExternalID, payment.AccountID, payment.CreatedAt,
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
func (r *PostgresPurchaseRepository) ReconcileExternal(ctx context.Context, tx ports.ExternalTransaction, accountID uuid.UUID) (bool, error) {
	query := `
		UPDATE payments SET external_id = $1, account_id = $6, status = 'PAID', paid_at = COALESCE(paid_at, $5::timestamptz)
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
	tag, err := r.db.Pool.Exec(ctx, query, tx.ID, tx.Kind, tx.Amount, tx.Date, tx.Date, accountID)
	if err != nil {
		return false, fmt.Errorf("erro ao conciliar transação externa: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *PostgresPurchaseRepository) SaveInvestments(ctx context.Context, itemID string, positions []ports.ExternalInvestment, day time.Time) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	seen := make([]string, 0, len(positions))
	for _, p := range positions {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `
			INSERT INTO investments (external_id, item_id, type, subtype, name, balance, amount, active, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE, NOW())
			ON CONFLICT (external_id) DO UPDATE SET
				item_id = EXCLUDED.item_id, type = EXCLUDED.type, subtype = EXCLUDED.subtype, name = EXCLUDED.name,
				balance = EXCLUDED.balance, amount = EXCLUDED.amount, active = TRUE, updated_at = NOW()
			RETURNING id
		`, p.ID, itemID, p.Type, p.Subtype, p.Name, p.Balance, p.Amount).Scan(&id)
		if err != nil {
			return fmt.Errorf("erro ao salvar posição de investimento: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO investment_balances (investment_id, day, balance) VALUES ($1, $2::date, $3)
			ON CONFLICT (investment_id, day) DO UPDATE SET balance = EXCLUDED.balance
		`, id, day, p.Balance); err != nil {
			return fmt.Errorf("erro ao salvar saldo da posição: %w", err)
		}
		seen = append(seen, p.ID)
	}

	// Posição que sumiu do Pluggy foi resgatada: zera o saldo daqui em diante para o histórico não carregá-la.
	if _, err := tx.Exec(ctx, `
		WITH gone AS (
			UPDATE investments SET active = FALSE, balance = 0, updated_at = NOW()
			WHERE item_id = $1 AND active AND external_id <> ALL($2::text[])
			RETURNING id
		)
		INSERT INTO investment_balances (investment_id, day, balance)
		SELECT id, $3::date, 0 FROM gone
		ON CONFLICT (investment_id, day) DO UPDATE SET balance = 0
	`, itemID, seen, day); err != nil {
		return fmt.Errorf("erro ao inativar posições resgatadas: %w", err)
	}
	return tx.Commit(ctx)
}
