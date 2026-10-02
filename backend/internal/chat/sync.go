package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

// "/sync" é o comando natural no Telegram; "sincronizar" funciona nos dois canais.
func isSyncCommand(text string) bool {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "/sync", "sincronizar":
		return true
	}
	return false
}

func (h *Handler) handleSync(ctx context.Context) {
	if h.syncer == nil {
		h.sendText(ctx, "ℹ️ O Open Finance não está configurado. Veja a seção Open Finance no README.")
		return
	}

	h.sendText(ctx, "🔄 Sincronizando com o Open Finance, aguarde...")
	result, err := h.syncer.Sync(ctx)
	if errors.Is(err, usecase.ErrNotConfigured) {
		h.sendText(ctx, "ℹ️ O Open Finance não está configurado. Veja a página Configurações do dashboard ou a seção Open Finance no README.")
		return
	}
	if errors.Is(err, usecase.ErrSyncInProgress) {
		h.sendText(ctx, "⏳ Já existe uma sincronização em andamento. Tente em instantes.")
		return
	}

	msg := fmt.Sprintf("✅ Sincronização concluída\n🆕 %d novo(s)\n🔗 %d conciliado(s) com lançamentos manuais\n♻️ %d já existiam",
		result.Inserted, result.Reconciled, result.Existing)
	if result.Positions > 0 {
		msg += fmt.Sprintf("\n📈 %d posição(ões) de investimento atualizada(s)", result.Positions)
	}
	if err != nil {
		h.logger.Error("erro na sincronização do Open Finance", "error", err)
		msg += "\n⚠️ Alguns itens falharam; veja os logs."
	}
	h.sendText(ctx, msg)
}
