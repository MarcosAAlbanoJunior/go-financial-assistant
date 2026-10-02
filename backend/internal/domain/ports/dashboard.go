package ports

// DashboardReader é tudo o que a API do dashboard lê e grava, composto pelos contratos de cada contexto. Quem consome só uma
// parte (ex.: Insights) deve declarar a parte de que precisa em vez de pedir o DashboardReader inteiro.
type DashboardReader interface {
	TotalsReader
	TransactionReader
	AccountReader
	PortfolioReader
	BudgetReader
	ReviewReader
	GoalStore
	CategoryStore
	CoachStore
}

// InsightsReader é o que as leituras da revisão, metas, economia, projeção, saldos e resumo semanal usam: orçamento,
// revisão, metas, contas, carteira e totais, sem o Coach, a classificação nem as transações.
type InsightsReader interface {
	TotalsReader
	AccountReader
	PortfolioReader
	BudgetReader
	ReviewReader
	GoalStore
}
