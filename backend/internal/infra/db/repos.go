package db

import "github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"

// Um repositório por contexto, todos sobre o mesmo pool. Cada um implementa só o contrato do seu contexto em ports/.

// TotalsRepo lê e grava totais por mês e divisão dos gastos.
type TotalsRepo struct{ db *DB }

// TransactionsRepo lê e grava transações com filtro, paginação e agrupamento.
type TransactionsRepo struct{ db *DB }

// AccountsRepo lê e grava contas, cartões e instituições.
type AccountsRepo struct{ db *DB }

// PortfolioRepo lê e grava posições de investimento e o histórico da carteira.
type PortfolioRepo struct{ db *DB }

// BudgetRepo lê e grava despesas por conta, regras de fixas/variáveis e renda.
type BudgetRepo struct{ db *DB }

// ReviewRepo lê e grava categorias por mês, dispensas e decisões "cancelei" da revisão.
type ReviewRepo struct{ db *DB }

// GoalsRepo lê e grava metas financeiras.
type GoalsRepo struct{ db *DB }

// CoachRepo lê e grava o histórico do Coach.
type CoachRepo struct{ db *DB }

// ClassifyRepo lê e grava a classificação das despesas em "Outros".
type ClassifyRepo struct{ db *DB }

// DashboardRepo reúne os repositórios de leitura do dashboard; os métodos de cada um são promovidos e ele satisfaz ports.DashboardReader.
type DashboardRepo struct {
	*TotalsRepo
	*TransactionsRepo
	*AccountsRepo
	*PortfolioRepo
	*BudgetRepo
	*ReviewRepo
	*GoalsRepo
	*CoachRepo
	*ClassifyRepo
}

func NewDashboardRepo(db *DB) *DashboardRepo {
	return &DashboardRepo{
		&TotalsRepo{db: db},
		&TransactionsRepo{db: db},
		&AccountsRepo{db: db},
		&PortfolioRepo{db: db},
		&BudgetRepo{db: db},
		&ReviewRepo{db: db},
		&GoalsRepo{db: db},
		&CoachRepo{db: db},
		&ClassifyRepo{db: db},
	}
}

func NewDashboardReader(db *DB) ports.DashboardReader { return NewDashboardRepo(db) }
