package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/setup"
)

// SetupStore guarda o dono do dashboard e o rascunho do setup (migration 019).
type SetupStore struct{ db *DB }

func NewSetupStore(db *DB) *SetupStore { return &SetupStore{db: db} }

func (s *SetupStore) LoadOwner(ctx context.Context) (setup.Owner, error) {
	var o setup.Owner
	var hash *string
	err := s.db.Pool.QueryRow(ctx, `SELECT password_hash, setup_completed_at, reopen_done FROM dashboard_owner WHERE id = 1`).
		Scan(&hash, &o.CompletedAt, &o.ReopenDone)
	if errors.Is(err, pgx.ErrNoRows) {
		return setup.Owner{}, nil
	}
	if err != nil {
		return setup.Owner{}, fmt.Errorf("erro ao ler o dono do dashboard: %w", err)
	}
	if hash != nil {
		o.PasswordHash = *hash
	}
	return o, nil
}

func (s *SetupStore) LoadDraft(ctx context.Context) (setup.Draft, error) {
	var d setup.Draft
	var hash, token, bot, name, username *string
	var candidate *int64
	err := s.db.Pool.QueryRow(ctx, `
		SELECT password_hash, telegram_token, telegram_bot, telegram_offset, candidate_id, candidate_name, candidate_username, candidate_accepted
		FROM setup_draft WHERE id = 1
	`).Scan(&hash, &token, &bot, &d.TelegramOffset, &candidate, &name, &username, &d.CandidateAccepted)
	if errors.Is(err, pgx.ErrNoRows) {
		return setup.Draft{}, nil
	}
	if err != nil {
		return setup.Draft{}, fmt.Errorf("erro ao ler o rascunho do setup: %w", err)
	}
	d.PasswordHash, d.TelegramToken, d.TelegramBot = orEmpty(hash), orEmpty(token), orEmpty(bot)
	if candidate != nil {
		d.Candidate = &setup.Candidate{ID: *candidate, Name: orEmpty(name), Username: orEmpty(username)}
	}
	return d, nil
}

func (s *SetupStore) SaveDraft(ctx context.Context, d setup.Draft) error {
	var candidate *int64
	var name, username *string
	if d.Candidate != nil {
		candidate, name, username = &d.Candidate.ID, &d.Candidate.Name, &d.Candidate.Username
	}
	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO setup_draft (id, password_hash, telegram_token, telegram_bot, telegram_offset, candidate_id, candidate_name, candidate_username, candidate_accepted, updated_at)
		VALUES (1, $1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (id) DO UPDATE SET
			password_hash = EXCLUDED.password_hash, telegram_token = EXCLUDED.telegram_token, telegram_bot = EXCLUDED.telegram_bot,
			telegram_offset = EXCLUDED.telegram_offset, candidate_id = EXCLUDED.candidate_id, candidate_name = EXCLUDED.candidate_name,
			candidate_username = EXCLUDED.candidate_username, candidate_accepted = EXCLUDED.candidate_accepted, updated_at = NOW()
	`, nullable(d.PasswordHash), nullable(d.TelegramToken), nullable(d.TelegramBot), d.TelegramOffset, candidate, name, username, d.CandidateAccepted)
	if err != nil {
		return fmt.Errorf("erro ao salvar o rascunho do setup: %w", err)
	}
	return nil
}

// Complete grava tudo numa transação e devolve quando concluiu. A linha do dono só é atualizada se ainda estiver como
// c.Expected: se outra conclusão chegou antes, nada é gravado e devolve nil.
func (s *SetupStore) Complete(ctx context.Context, c setup.Completion) (*time.Time, error) {
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("erro ao concluir o setup: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `INSERT INTO dashboard_owner (id) VALUES (1) ON CONFLICT (id) DO NOTHING`); err != nil {
		return nil, fmt.Errorf("erro ao concluir o setup: %w", err)
	}
	var at time.Time
	err = tx.QueryRow(ctx, `
		UPDATE dashboard_owner
		SET password_hash = COALESCE($1, password_hash), setup_completed_at = NOW(), reopen_done = reopen_done OR $2, updated_at = NOW()
		WHERE id = 1 AND setup_completed_at IS NOT DISTINCT FROM $3 AND reopen_done = $4
		RETURNING setup_completed_at
	`, nullable(c.PasswordHash), c.Reopen, c.Expected.CompletedAt, c.Expected.ReopenDone).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("erro ao concluir o setup: %w", err)
	}
	for _, r := range c.Rows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO settings (key, value, secret, updated_at) VALUES ($1, $2, $3, NOW())
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, secret = EXCLUDED.secret, updated_at = NOW()
		`, r.Key, r.Value, r.Secret); err != nil {
			return nil, fmt.Errorf("erro ao gravar o canal: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM setup_draft`); err != nil {
		return nil, fmt.Errorf("erro ao apagar o rascunho do setup: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("erro ao concluir o setup: %w", err)
	}
	return &at, nil
}

func (s *SetupStore) ClearReopen(ctx context.Context) error {
	if _, err := s.db.Pool.Exec(ctx, `UPDATE dashboard_owner SET reopen_done = FALSE, updated_at = NOW() WHERE id = 1`); err != nil {
		return fmt.Errorf("erro ao liberar a reabertura do setup: %w", err)
	}
	return nil
}

func orEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
