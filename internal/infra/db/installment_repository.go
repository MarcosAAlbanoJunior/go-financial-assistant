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

type installmentPurchaseModel struct {
	ID                uuid.UUID `db:"id"`
	Description       string    `db:"description"`
	TotalAmount       float64   `db:"total_amount"`
	InstallmentAmount float64   `db:"installment_amount"`
	TotalInstallments int       `db:"total_installments"`
	Category          string    `db:"category"`
	Payment           string    `db:"payment"`
	PurchaseDate      time.Time `db:"purchase_date"`
	RawInput          string    `db:"raw_input"`
	CreatedAt         time.Time `db:"created_at"`
}

func (m installmentPurchaseModel) toDomain() *domain.InstallmentPurchase {
	return &domain.InstallmentPurchase{
		ID:                m.ID,
		Description:       m.Description,
		TotalAmount:       m.TotalAmount,
		InstallmentAmount: m.InstallmentAmount,
		TotalInstallments: m.TotalInstallments,
		Category:          domain.Category(m.Category),
		Payment:           domain.PaymentMethod(m.Payment),
		PurchaseDate:      m.PurchaseDate,
		RawInput:          m.RawInput,
		CreatedAt:         m.CreatedAt,
	}
}

type installmentModel struct {
	ID                uuid.UUID  `db:"id"`
	PurchaseID        uuid.UUID  `db:"purchase_id"`
	InstallmentNumber int        `db:"installment_number"`
	TotalInstallments int        `db:"total_installments"`
	Amount            float64    `db:"amount"`
	DueDate           time.Time  `db:"due_date"`
	PaidAt            *time.Time `db:"paid_at"`
	Status            string     `db:"status"`
	CreatedAt         time.Time  `db:"created_at"`
}

func (m installmentModel) toDomain() domain.Installment {
	return domain.Installment{
		ID:                m.ID,
		PurchaseID:        m.PurchaseID,
		InstallmentNumber: m.InstallmentNumber,
		TotalInstallments: m.TotalInstallments,
		Amount:            m.Amount,
		DueDate:           m.DueDate,
		PaidAt:            m.PaidAt,
		Status:            domain.InstallmentStatus(m.Status),
		CreatedAt:         m.CreatedAt,
	}
}

type PostgresInstallmentRepository struct {
	db *DB
}

func NewInstallmentRepository(db *DB) ports.InstallmentRepository {
	return &PostgresInstallmentRepository{db: db}
}

func (r *PostgresInstallmentRepository) SavePurchase(
	ctx context.Context,
	purchase *domain.InstallmentPurchase,
	installments []domain.Installment,
) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	purchaseQuery := `
		INSERT INTO installment_purchases
			(id, description, total_amount, installment_amount, total_installments,
			 category, payment, purchase_date, raw_input, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	if _, err := tx.Exec(ctx, purchaseQuery,
		purchase.ID, purchase.Description, purchase.TotalAmount,
		purchase.InstallmentAmount, purchase.TotalInstallments,
		purchase.Category, purchase.Payment, purchase.PurchaseDate,
		purchase.RawInput, purchase.CreatedAt,
	); err != nil {
		return fmt.Errorf("erro ao salvar compra parcelada: %w", err)
	}

	installmentQuery := `
		INSERT INTO installments
			(id, purchase_id, installment_number, total_installments, amount, due_date, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	for _, inst := range installments {
		if _, err := tx.Exec(ctx, installmentQuery,
			inst.ID, inst.PurchaseID, inst.InstallmentNumber, inst.TotalInstallments,
			inst.Amount, inst.DueDate, inst.Status, inst.CreatedAt,
		); err != nil {
			return fmt.Errorf("erro ao salvar parcela %d: %w", inst.InstallmentNumber, err)
		}
	}

	return tx.Commit(ctx)
}

func (r *PostgresInstallmentRepository) FindPurchaseByID(ctx context.Context, id uuid.UUID) (*domain.InstallmentPurchase, error) {
	query := `
		SELECT id, description, total_amount, installment_amount, total_installments,
		       category, payment, purchase_date, raw_input, created_at
		FROM installment_purchases
		WHERE id = $1
	`

	rows, err := r.db.Pool.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar compra parcelada: %w", err)
	}

	model, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[installmentPurchaseModel])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInstallmentPurchaseNotFound
		}
		return nil, fmt.Errorf("erro ao escanear compra parcelada: %w", err)
	}

	return model.toDomain(), nil
}

func (r *PostgresInstallmentRepository) FindInstallmentsByPurchaseID(ctx context.Context, purchaseID uuid.UUID) ([]domain.Installment, error) {
	query := `
		SELECT id, purchase_id, installment_number, total_installments, amount, due_date, paid_at, status, created_at
		FROM installments
		WHERE purchase_id = $1
		ORDER BY installment_number
	`

	rows, err := r.db.Pool.Query(ctx, query, purchaseID)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar parcelas: %w", err)
	}

	models, err := pgx.CollectRows(rows, pgx.RowToStructByName[installmentModel])
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear parcelas: %w", err)
	}

	result := make([]domain.Installment, len(models))
	for i, m := range models {
		result[i] = m.toDomain()
	}
	return result, nil
}

func (r *PostgresInstallmentRepository) FindPendingInstallments(ctx context.Context) ([]domain.Installment, error) {
	query := `
		SELECT id, purchase_id, installment_number, total_installments, amount, due_date, paid_at, status, created_at
		FROM installments
		WHERE status = 'PENDING'
		ORDER BY due_date
	`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar parcelas pendentes: %w", err)
	}

	models, err := pgx.CollectRows(rows, pgx.RowToStructByName[installmentModel])
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear parcelas pendentes: %w", err)
	}

	result := make([]domain.Installment, len(models))
	for i, m := range models {
		result[i] = m.toDomain()
	}
	return result, nil
}
