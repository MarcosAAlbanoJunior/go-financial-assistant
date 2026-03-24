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

type expenseModel struct {
	ID                 uuid.UUID  `db:"id"`
	Amount             float64    `db:"amount"`
	Description        string     `db:"description"`
	Category           string     `db:"category"`
	Payment            string     `db:"payment"`
	ReceiptURL         *string    `db:"receipt_url"`
	RawInput           string     `db:"raw_input"`
	RecurringExpenseID *uuid.UUID `db:"recurring_expense_id"`
	CreatedAt          time.Time  `db:"created_at"`
}

func (m expenseModel) toDomain() *domain.Expense {
	return &domain.Expense{
		ID:                 m.ID,
		Amount:             m.Amount,
		Description:        m.Description,
		Category:           domain.Category(m.Category),
		Payment:            domain.PaymentMethod(m.Payment),
		ReceiptURL:         m.ReceiptURL,
		RawInput:           m.RawInput,
		RecurringExpenseID: m.RecurringExpenseID,
		CreatedAt:          m.CreatedAt,
	}
}

type PostgresExpenseRepository struct {
	db *DB
}

func NewExpenseRepository(db *DB) ports.ExpenseRepository {
	return &PostgresExpenseRepository{db: db}
}

func (r *PostgresExpenseRepository) Save(ctx context.Context, expense *domain.Expense) error {
	query := `
        INSERT INTO expenses (id, amount, description, category, payment, receipt_url, raw_input, recurring_expense_id, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `

	_, err := r.db.Pool.Exec(ctx, query,
		expense.ID,
		expense.Amount,
		expense.Description,
		expense.Category,
		expense.Payment,
		expense.ReceiptURL,
		expense.RawInput,
		expense.RecurringExpenseID,
		expense.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("erro ao salvar expense: %w", err)
	}

	return nil
}

func (r *PostgresExpenseRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Expense, error) {
	query := `
        SELECT id, amount, description, category, payment, receipt_url, raw_input, recurring_expense_id, created_at
        FROM expenses
        WHERE id = $1
    `

	row, err := r.db.Pool.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar expense: %w", err)
	}

	model, err := pgx.CollectOneRow(row, pgx.RowToStructByName[expenseModel])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrExpenseNotFound
		}
		return nil, fmt.Errorf("erro ao escanear expense: %w", err)
	}

	return model.toDomain(), nil
}

func (r *PostgresExpenseRepository) FindAll(ctx context.Context, filter ports.ExpenseFilter) ([]domain.Expense, error) {
	query := `
        SELECT id, amount, description, category, payment, receipt_url, raw_input, recurring_expense_id, created_at
        FROM expenses
        WHERE 1=1
    `
	args := []any{}
	argIdx := 1

	if filter.Category != nil {
		query += fmt.Sprintf(" AND category = $%d", argIdx)
		args = append(args, *filter.Category)
		argIdx++
	}

	if filter.Payment != nil {
		query += fmt.Sprintf(" AND payment = $%d", argIdx)
		args = append(args, *filter.Payment)
		argIdx++
	}

	if filter.FromDate != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *filter.FromDate)
		argIdx++
	}

	if filter.ToDate != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *filter.ToDate)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar expenses: %w", err)
	}

	models, err := pgx.CollectRows(rows, pgx.RowToStructByName[expenseModel])
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear expenses: %w", err)
	}

	expenses := make([]domain.Expense, len(models))
	for i, m := range models {
		expenses[i] = *m.toDomain()
	}

	return expenses, nil
}

func (r *PostgresExpenseRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM expenses WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("erro ao deletar expense: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrExpenseNotFound
	}

	return nil
}
