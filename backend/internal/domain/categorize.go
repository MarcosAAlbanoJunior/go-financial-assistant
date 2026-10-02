package domain

import "time"

// UncategorizedGroup soma as despesas em "Outros" de uma mesma conta (descrição normalizada em Key).
type UncategorizedGroup struct {
	Key   string
	Label string // descrição de exemplo, como veio do banco
	Count int
	Total float64
	Last  time.Time
}

// CategoryRule é a categoria escolhida para uma conta; OTHER significa "manter em Outros".
type CategoryRule struct {
	Key      string
	Category string
}
