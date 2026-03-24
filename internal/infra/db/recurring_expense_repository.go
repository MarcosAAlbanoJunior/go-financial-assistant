package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type recurringExpenseModel struct {
	ID                 uuid.UUID  `db:"id"`
	Description        string     `db:"description"`
	Amount             float64    `db:"amount"`
	Category           string     `db:"category"`
	Payment            string     `db:"payment"`
	DayOfMonth         int        `db:"day_of_month"`
	StartDate          time.Time  `db:"start_date"`
	EndDate            *time.Time `db:"end_date"`
	IsActive           bool       `db:"is_active"`
	LastGeneratedDate  *time.Time `db:"last_generated_date"`
	CancelledAt        *time.Time `db:"cancelled_at"`
	CancellationReason *string    `db:"cancellation_reason"`
	RawInput           string     `db:"raw_input"`
	CreatedAt          time.Time  `db:"created_at"`
}

func (m recurringExpenseModel) toDomain() *domain.RecurringExpense {
	return &domain.RecurringExpense{
		ID:                 m.ID,
		Description:        m.Description,
		Amount:             m.Amount,
		Category:           domain.Category(m.Category),
		Payment:            domain.PaymentMethod(m.Payment),
		DayOfMonth:         m.DayOfMonth,
		StartDate:          m.StartDate,
		EndDate:            m.EndDate,
		IsActive:           m.IsActive,
		LastGeneratedDate:  m.LastGeneratedDate,
		CancelledAt:        m.CancelledAt,
		CancellationReason: m.CancellationReason,
		RawInput:           m.RawInput,
		CreatedAt:          m.CreatedAt,
	}
}

type PostgresRecurringExpenseRepository struct {
	db *DB
}

func NewRecurringExpenseRepository(db *DB) ports.RecurringExpenseRepository {
	return &PostgresRecurringExpenseRepository{db: db}
}

func (r *PostgresRecurringExpenseRepository) Save(ctx context.Context, expense *domain.RecurringExpense) error {
	query := `
		INSERT INTO recurring_expenses
			(id, description, amount, category, payment, day_of_month, start_date,
			 is_active, raw_input, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Pool.Exec(ctx, query,
		expense.ID, expense.Description, expense.Amount,
		expense.Category, expense.Payment, expense.DayOfMonth,
		expense.StartDate, expense.IsActive, expense.RawInput, expense.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("erro ao salvar despesa recorrente: %w", err)
	}
	return nil
}

func (r *PostgresRecurringExpenseRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.RecurringExpense, error) {
	query := `
		SELECT id, description, amount, category, payment, day_of_month, start_date,
		       end_date, is_active, last_generated_date, cancelled_at, cancellation_reason,
		       raw_input, created_at
		FROM recurring_expenses
		WHERE id = $1
	`
	rows, err := r.db.Pool.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar despesa recorrente: %w", err)
	}

	model, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[recurringExpenseModel])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRecurringExpenseNotFound
		}
		return nil, fmt.Errorf("erro ao escanear despesa recorrente: %w", err)
	}
	return model.toDomain(), nil
}

func (r *PostgresRecurringExpenseRepository) FindActive(ctx context.Context) ([]domain.RecurringExpense, error) {
	query := `
		SELECT id, description, amount, category, payment, day_of_month, start_date,
		       end_date, is_active, last_generated_date, cancelled_at, cancellation_reason,
		       raw_input, created_at
		FROM recurring_expenses
		WHERE is_active = TRUE
		ORDER BY description
	`
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar despesas recorrentes ativas: %w", err)
	}

	models, err := pgx.CollectRows(rows, pgx.RowToStructByName[recurringExpenseModel])
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear despesas recorrentes: %w", err)
	}

	result := make([]domain.RecurringExpense, len(models))
	for i, m := range models {
		result[i] = *m.toDomain()
	}
	return result, nil
}

func (r *PostgresRecurringExpenseRepository) Update(ctx context.Context, expense *domain.RecurringExpense) error {
	query := `
		UPDATE recurring_expenses
		SET description = $2, amount = $3, category = $4, payment = $5,
		    day_of_month = $6, end_date = $7, is_active = $8,
		    last_generated_date = $9, cancelled_at = $10, cancellation_reason = $11
		WHERE id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query,
		expense.ID, expense.Description, expense.Amount,
		expense.Category, expense.Payment, expense.DayOfMonth,
		expense.EndDate, expense.IsActive, expense.LastGeneratedDate,
		expense.CancelledAt, expense.CancellationReason,
	)
	if err != nil {
		return fmt.Errorf("erro ao atualizar despesa recorrente: %w", err)
	}
	return nil
}

func (r *PostgresRecurringExpenseRepository) FindByDescription(ctx context.Context, description string) ([]domain.RecurringExpense, error) {
	query := `
		SELECT id, description, amount, category, payment, day_of_month, start_date,
		       end_date, is_active, last_generated_date, cancelled_at, cancellation_reason,
		       raw_input, created_at
		FROM recurring_expenses
		WHERE is_active = TRUE AND description ILIKE $1
		ORDER BY description
	`
	rows, err := r.db.Pool.Query(ctx, query, "%"+description+"%")
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar despesas recorrentes por descrição: %w", err)
	}

	models, err := pgx.CollectRows(rows, pgx.RowToStructByName[recurringExpenseModel])
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear despesas recorrentes: %w", err)
	}

	result := make([]domain.RecurringExpense, len(models))
	for i, m := range models {
		result[i] = *m.toDomain()
	}
	return result, nil
}
