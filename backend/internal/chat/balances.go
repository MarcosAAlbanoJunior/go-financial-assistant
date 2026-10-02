package chat

import (
	"context"
	"strings"
	"time"
)

// Balancer monta o painel de saldos em texto, com o mesmo cálculo da tela do dashboard.
type Balancer interface {
	BalancesText(ctx context.Context, now time.Time) (string, error)
}

// SetBalancer habilita o comando "/saldos".
func (h *Handler) SetBalancer(b Balancer) { h.balancer = b }

// "/saldos" é o comando natural no Telegram; "saldos" e "painel" valem nos dois canais.
func isBalancesCommand(text string) bool {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "/saldos", "saldos", "painel":
		return true
	}
	return false
}

func (h *Handler) handleBalances(ctx context.Context) {
	if h.balancer == nil {
		h.sendText(ctx, "ℹ️ O painel de saldos não está disponível.")
		return
	}
	text, err := h.balancer.BalancesText(ctx, time.Now())
	if err != nil {
		h.logger.Error("erro ao montar os saldos", "error", err)
		h.sendText(ctx, "Não consegui montar os saldos agora. Tente de novo em instantes.")
		return
	}
	h.sendText(ctx, text)
}
