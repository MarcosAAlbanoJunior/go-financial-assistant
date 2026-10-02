package db

import (
	"context"
	"fmt"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SettingsStore guarda as configurações editadas no dashboard.
type SettingsStore struct{ db *DB }

func NewSettingsStore(db *DB) *SettingsStore { return &SettingsStore{db: db} }

func (s *SettingsStore) LoadSettings(ctx context.Context) ([]settings.Row, error) {
	rows, err := s.db.Pool.Query(ctx, `SELECT key, value, secret FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler configurações: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (settings.Row, error) {
		var r settings.Row
		err := row.Scan(&r.Key, &r.Value, &r.Secret)
		return r, err
	})
}

func (s *SettingsStore) SaveSetting(ctx context.Context, r settings.Row) error {
	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO settings (key, value, secret, updated_at) VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, secret = EXCLUDED.secret, updated_at = NOW()
	`, r.Key, r.Value, r.Secret)
	if err != nil {
		return fmt.Errorf("erro ao salvar configuração: %w", err)
	}
	return nil
}

func (s *SettingsStore) DeleteSetting(ctx context.Context, key string) error {
	if _, err := s.db.Pool.Exec(ctx, `DELETE FROM settings WHERE key = $1`, key); err != nil {
		return fmt.Errorf("erro ao apagar configuração: %w", err)
	}
	return nil
}

// OwnTransferCandidates lista os lançamentos do banco que parecem transferência (Pix, TED, DOC) e ainda contam;
// quem chama decide, pelo nome, quais são entre contas da própria pessoa.
func (r *PostgresPurchaseRepository) OwnTransferCandidates(ctx context.Context) ([]domain.TransferCandidate, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT pay.id, p.description, p.kind, pay.amount
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE pay.external_id IS NOT NULL AND pay.status <> 'CANCELLED' AND p.kind IN ('EXPENSE', 'INCOME')
		  AND p.description ~* '(pix|transfer|ted|doc)'
	`)
	if err != nil {
		return nil, fmt.Errorf("erro ao procurar transferências: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.TransferCandidate, error) {
		var c domain.TransferCandidate
		err := row.Scan(&c.PaymentID, &c.Description, &c.Kind, &c.Amount)
		return c, err
	})
}

// CancelPayments marca os pagamentos como cancelados (deixam de contar em tudo).
func (r *PostgresPurchaseRepository) CancelPayments(ctx context.Context, ids []uuid.UUID) (int64, error) {
	tag, err := r.db.Pool.Exec(ctx, `UPDATE payments SET status = 'CANCELLED' WHERE id = ANY($1) AND status <> 'CANCELLED'`, ids)
	if err != nil {
		return 0, fmt.Errorf("erro ao cancelar lançamentos: %w", err)
	}
	return tag.RowsAffected(), nil
}

// NewTransferCleaner devolve quem acha e cancela transferências entre contas da própria pessoa.
func NewTransferCleaner(db *DB) ports.TransferCleaner { return &PostgresPurchaseRepository{db: db} }

func (s *SettingsStore) RecordAudit(ctx context.Context, e settings.AuditEntry) error {
	_, err := s.db.Pool.Exec(ctx, `INSERT INTO settings_audit (action, key, sensitive, ip) VALUES ($1, $2, $3, $4)`, e.Action, e.Key, e.Sensitive, e.IP)
	if err != nil {
		return fmt.Errorf("erro ao registrar alteração: %w", err)
	}
	return nil
}

func (s *SettingsStore) RecentAudit(ctx context.Context, limit int) ([]settings.AuditEntry, error) {
	rows, err := s.db.Pool.Query(ctx, `SELECT at, action, key, sensitive, ip FROM settings_audit ORDER BY at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler o histórico de alterações: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (settings.AuditEntry, error) {
		var e settings.AuditEntry
		err := row.Scan(&e.At, &e.Action, &e.Key, &e.Sensitive, &e.IP)
		return e, err
	})
}
