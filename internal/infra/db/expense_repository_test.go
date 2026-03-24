//go:build integration

package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

func setupDB(t *testing.T) *PostgresExpenseRepository {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL não definida — necessária para testes de integração")
	}
	db, err := NewPostgres(context.Background(), dsn)
	if err != nil {
		t.Fatalf("falha ao conectar no banco: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &PostgresExpenseRepository{db: db}
}

func newExpense(t *testing.T) *domain.Expense {
	t.Helper()
	e, err := domain.NewExpense(50.0, "Teste", domain.CategoryFood, domain.PaymentMethodPix, "raw", nil)
	if err != nil {
		t.Fatalf("falha ao criar expense: %v", err)
	}
	return e
}

func TestSave_AndFindByID(t *testing.T) {
	repo := setupDB(t)
	ctx := context.Background()

	expense := newExpense(t)
	t.Cleanup(func() { repo.Delete(ctx, expense.ID) })

	if err := repo.Save(ctx, expense); err != nil {
		t.Fatalf("Save: %v", err)
	}

	found, err := repo.FindByID(ctx, expense.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if found.ID != expense.ID {
		t.Errorf("ID diferente: %v != %v", found.ID, expense.ID)
	}
	if found.Amount != expense.Amount {
		t.Errorf("Amount diferente: %v != %v", found.Amount, expense.Amount)
	}
	if found.Description != expense.Description {
		t.Errorf("Description diferente")
	}
	if found.Category != expense.Category {
		t.Errorf("Category diferente")
	}
	if found.Payment != expense.Payment {
		t.Errorf("Payment diferente")
	}
}

func TestFindByID_NotFound(t *testing.T) {
	repo := setupDB(t)

	_, err := repo.FindByID(context.Background(), uuid.New())
	if err != domain.ErrExpenseNotFound {
		t.Errorf("esperava ErrExpenseNotFound, got: %v", err)
	}
}

func TestDelete_Success(t *testing.T) {
	repo := setupDB(t)
	ctx := context.Background()

	expense := newExpense(t)
	if err := repo.Save(ctx, expense); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := repo.Delete(ctx, expense.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.FindByID(ctx, expense.ID)
	if err != domain.ErrExpenseNotFound {
		t.Errorf("esperava ErrExpenseNotFound após delete, got: %v", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	repo := setupDB(t)

	err := repo.Delete(context.Background(), uuid.New())
	if err != domain.ErrExpenseNotFound {
		t.Errorf("esperava ErrExpenseNotFound, got: %v", err)
	}
}

func TestFindAll_NoFilter(t *testing.T) {
	repo := setupDB(t)
	ctx := context.Background()

	e1 := newExpense(t)
	e2 := newExpense(t)
	t.Cleanup(func() {
		repo.Delete(ctx, e1.ID)
		repo.Delete(ctx, e2.ID)
	})

	repo.Save(ctx, e1)
	repo.Save(ctx, e2)

	results, err := repo.FindAll(ctx, ports.ExpenseFilter{})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}

	found := 0
	for _, r := range results {
		if r.ID == e1.ID || r.ID == e2.ID {
			found++
		}
	}
	if found != 2 {
		t.Errorf("esperava encontrar 2 expenses, found %d", found)
	}
}

func TestFindAll_FilterByCategory(t *testing.T) {
	repo := setupDB(t)
	ctx := context.Background()

	food, _ := domain.NewExpense(10.0, "Lanche", domain.CategoryFood, domain.PaymentMethodCash, "raw", nil)
	transport, _ := domain.NewExpense(20.0, "Uber", domain.CategoryTransport, domain.PaymentMethodPix, "raw", nil)
	t.Cleanup(func() {
		repo.Delete(ctx, food.ID)
		repo.Delete(ctx, transport.ID)
	})

	repo.Save(ctx, food)
	repo.Save(ctx, transport)

	cat := domain.CategoryFood
	results, err := repo.FindAll(ctx, ports.ExpenseFilter{Category: &cat})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}

	for _, r := range results {
		if r.Category != domain.CategoryFood {
			t.Errorf("resultado com category inesperada: %s", r.Category)
		}
	}
}

func TestFindAll_FilterByPayment(t *testing.T) {
	repo := setupDB(t)
	ctx := context.Background()

	e, _ := domain.NewExpense(30.0, "Mercado", domain.CategoryShopping, domain.PaymentMethodDebitCard, "raw", nil)
	t.Cleanup(func() { repo.Delete(ctx, e.ID) })
	repo.Save(ctx, e)

	payment := domain.PaymentMethodDebitCard
	results, err := repo.FindAll(ctx, ports.ExpenseFilter{Payment: &payment})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}

	for _, r := range results {
		if r.Payment != domain.PaymentMethodDebitCard {
			t.Errorf("resultado com payment inesperado: %s", r.Payment)
		}
	}
}

func TestFindAll_FilterByDateRange(t *testing.T) {
	repo := setupDB(t)
	ctx := context.Background()

	e := newExpense(t)
	t.Cleanup(func() { repo.Delete(ctx, e.ID) })
	repo.Save(ctx, e)

	from := time.Now().Add(-time.Minute)
	to := time.Now().Add(time.Minute)
	results, err := repo.FindAll(ctx, ports.ExpenseFilter{FromDate: &from, ToDate: &to})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}

	found := false
	for _, r := range results {
		if r.ID == e.ID {
			found = true
		}
	}
	if !found {
		t.Error("expense não encontrada no filtro de datas")
	}
}

func TestExpenseModel_ToDomain(t *testing.T) {
	id := uuid.New()
	url := "https://example.com/receipt.jpg"
	now := time.Now().UTC().Truncate(time.Second)

	m := expenseModel{
		ID:          id,
		Amount:      99.9,
		Description: "Farmácia",
		Category:    "HEALTH",
		Payment:     "CREDIT_CARD",
		ReceiptURL:  &url,
		RawInput:    "raw",
		CreatedAt:   now,
	}

	e := m.toDomain()

	if e.ID != id {
		t.Errorf("ID diferente")
	}
	if e.Amount != 99.9 {
		t.Errorf("Amount diferente")
	}
	if e.Category != domain.CategoryHealth {
		t.Errorf("Category diferente: %s", e.Category)
	}
	if e.Payment != domain.PaymentMethodCreditCard {
		t.Errorf("Payment diferente: %s", e.Payment)
	}
	if e.ReceiptURL != &url {
		t.Error("ReceiptURL diferente")
	}
	if !e.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt diferente")
	}
}
