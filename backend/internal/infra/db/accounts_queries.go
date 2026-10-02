package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresPurchaseRepository) Accounts(ctx context.Context) ([]domain.Account, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, item_id, institution_id, type, name, last4, balance, credit_limit, available_credit_limit,
			brand, close_date, due_date, minimum_payment, auto_invested_balance, updated_at
		FROM accounts
		ORDER BY type, name
	`)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar contas: %w", err)
	}
	result, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Account, error) {
		var a domain.Account
		err := row.Scan(&a.ID, &a.ItemID, &a.InstitutionID, &a.Type, &a.Name, &a.Last4, &a.Balance, &a.CreditLimit, &a.AvailableCreditLimit,
			&a.Brand, &a.CloseDate, &a.DueDate, &a.MinimumPayment, &a.AutoInvested, &a.UpdatedAt)
		return a, err
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear contas: %w", err)
	}
	return result, nil
}

func (r *PostgresPurchaseRepository) Institutions(ctx context.Context) ([]domain.Institution, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, item_id, name, COALESCE(color, ''), logo IS NOT NULL FROM institutions ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar instituições: %w", err)
	}
	result, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Institution, error) {
		var i domain.Institution
		err := row.Scan(&i.ID, &i.ItemID, &i.Name, &i.Color, &i.HasLogo)
		return i, err
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear instituições: %w", err)
	}
	return result, nil
}

func (r *PostgresPurchaseRepository) InstitutionLogo(ctx context.Context, id uuid.UUID) ([]byte, string, bool, error) {
	var (
		data []byte
		mime *string
	)
	err := r.db.Pool.QueryRow(ctx, `SELECT logo, logo_mime FROM institutions WHERE id = $1`, id).Scan(&data, &mime)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (data == nil || mime == nil)) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, fmt.Errorf("erro ao ler logo da instituição: %w", err)
	}
	return data, *mime, true, nil
}
