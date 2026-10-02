package chat

import (
	"context"
	"strings"
	"time"
)

// Digester monta o resumo semanal sob demanda.
type Digester interface {
	WeeklyDigest(ctx context.Context, today time.Time) (string, error)
}

// SetDigester habilita o comando "/resumo".
func (h *Handler) SetDigester(d Digester) { h.digester = d }

// "/resumo" é o comando natural no Telegram; "resumo semanal" funciona nos dois canais. Frases como
// "resumo do mês" continuam indo para o assistente.
func isDigestCommand(text string) bool {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "/resumo", "resumo semanal":
		return true
	}
	return false
}

func (h *Handler) handleDigest(ctx context.Context) {
	if h.digester == nil {
		h.sendText(ctx, "ℹ️ O resumo semanal não está disponível.")
		return
	}
	text, err := h.digester.WeeklyDigest(ctx, time.Now())
	if err != nil {
		h.logger.Error("erro ao montar o resumo semanal", "error", err)
		h.sendText(ctx, "Não consegui montar o resumo agora. Tente de novo em instantes.")
		return
	}
	h.sendText(ctx, text)
}
