package domain

import (
	"time"

	"github.com/google/uuid"
)

// Position é uma posição de investimento ativa, com o saldo da última sincronização.
type Position struct {
	ID        uuid.UUID
	Type      string
	Subtype   string
	Name      string
	Balance   float64
	Amount    float64
	UpdatedAt time.Time
}

// PortfolioMonth é o saldo total das posições ao fim do mês; nil quando não há como saber.
// Estimated indica que o valor foi reconstruído pelas movimentações (meses anteriores à
// primeira sincronização); a partir dela o saldo é o exato informado pelo banco.
type PortfolioMonth struct {
	Month     time.Time
	Balance   *float64
	Estimated bool
}
