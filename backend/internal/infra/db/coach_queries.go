package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

func (r *CoachRepo) SaveCoachAnalysis(ctx context.Context, a domain.CoachAnalysis) error {
	answers, err := json.Marshal(a.Answers)
	if err != nil {
		return fmt.Errorf("erro ao serializar respostas do coach: %w", err)
	}
	if a.Answers == nil {
		answers = []byte("{}")
	}
	if _, err := r.db.Pool.Exec(ctx, `INSERT INTO coach_analyses (id, month, advice, answers) VALUES ($1, $2, $3, $4)`, a.ID, a.Month, a.Advice, answers); err != nil {
		return fmt.Errorf("erro ao gravar análise do coach: %w", err)
	}
	return nil
}

func (r *CoachRepo) CoachAnalyses(ctx context.Context, month *time.Time, limit int) ([]domain.CoachAnalysis, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, month, created_at, advice, answers FROM coach_analyses
		WHERE $1::date IS NULL OR month = $1::date
		ORDER BY created_at DESC LIMIT $2
	`, month, limit)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler análises do coach: %w", err)
	}
	defer rows.Close()

	var result []domain.CoachAnalysis
	for rows.Next() {
		var a domain.CoachAnalysis
		var answers []byte
		if err := rows.Scan(&a.ID, &a.Month, &a.CreatedAt, &a.Advice, &answers); err != nil {
			return nil, fmt.Errorf("erro ao escanear análise do coach: %w", err)
		}
		if err := json.Unmarshal(answers, &a.Answers); err != nil {
			return nil, fmt.Errorf("erro ao ler respostas do coach: %w", err)
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func (r *CoachRepo) SetCoachAnswer(ctx context.Context, id uuid.UUID, key, answer string) (bool, error) {
	// A chave e a resposta vão como parâmetros do jsonb, nunca concatenadas no SQL.
	query, args := `UPDATE coach_analyses SET answers = answers || jsonb_build_object($2::text, $3::text) WHERE id = $1`, []any{id, key, answer}
	if answer == "" {
		query, args = `UPDATE coach_analyses SET answers = answers - $2::text WHERE id = $1`, []any{id, key}
	}
	tag, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("erro ao gravar resposta do coach: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *CoachRepo) DeleteCoachAnalysis(ctx context.Context, id uuid.UUID) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM coach_analyses WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("erro ao apagar análise do coach: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
