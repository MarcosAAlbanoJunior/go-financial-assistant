package db

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

func (r *GoalsRepo) Goals(ctx context.Context) ([]domain.Goal, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, kind, name, COALESCE(target_amount, 0), COALESCE(target_date, '0001-01-01'), COALESCE(category, ''),
		       COALESCE(cut_percent, 0), COALESCE(baseline, 0), COALESCE(reserve_months, 0), created_at
		FROM goals ORDER BY created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler metas: %w", err)
	}
	defer rows.Close()

	var result []domain.Goal
	for rows.Next() {
		var g domain.Goal
		if err := rows.Scan(&g.ID, &g.Kind, &g.Name, &g.TargetAmount, &g.TargetDate, &g.Category, &g.CutPercent, &g.Baseline, &g.ReserveMonths, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("erro ao escanear meta: %w", err)
		}
		if g.Kind != domain.GoalSave {
			g.TargetDate = time.Time{}
		}
		result = append(result, g)
	}
	return result, rows.Err()
}

func (r *GoalsRepo) CreateGoal(ctx context.Context, g domain.Goal) error {
	// Só os campos do tipo da meta são gravados; os demais ficam nulos.
	var amount, baseline *float64
	var date *time.Time
	var category *string
	var cut, reserve *int
	switch g.Kind {
	case domain.GoalSave:
		amount, date = &g.TargetAmount, &g.TargetDate
	case domain.GoalCut:
		category, cut, baseline = &g.Category, &g.CutPercent, &g.Baseline
	case domain.GoalReserve:
		reserve = &g.ReserveMonths
	}
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO goals (id, kind, name, target_amount, target_date, category, cut_percent, baseline, reserve_months)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, g.ID, string(g.Kind), g.Name, amount, date, category, cut, baseline, reserve)
	if err != nil {
		return fmt.Errorf("erro ao gravar meta: %w", err)
	}
	return nil
}

func (r *GoalsRepo) DeleteGoal(ctx context.Context, id uuid.UUID) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM goals WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("erro ao apagar meta: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
