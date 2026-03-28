package httpserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"
)

func (h *webhookHandler) handleExportCommand(ctx context.Context, w http.ResponseWriter, month time.Time) {
	data, filename, err := h.csvExporter.Execute(ctx, month)
	if err != nil {
		h.logger.Error("erro ao gerar CSV", "error", err)
		h.sendText(ctx, "❌ Não consegui gerar a planilha. Tente novamente.")
		h.writeError(w, "erro ao gerar CSV", http.StatusInternalServerError)
		return
	}

	if data == nil {
		h.sendText(ctx, fmt.Sprintf("📊 Sem despesas registradas em %s.", month.Format("01/2006")))
		w.WriteHeader(http.StatusOK)
		return
	}

	caption := fmt.Sprintf("📊 Planilha de despesas de %s!", month.Format("01/2006"))
	base64Data := base64.StdEncoding.EncodeToString(data)

	if sentID, err := h.messenger.SendDocument(ctx, h.ownerPhone, filename, base64Data, caption); err != nil {
		h.logger.Error("erro ao enviar documento", "error", err)
	} else if sentID != "" {
		h.sentIDs.Store(sentID, time.Now())
	}

	w.WriteHeader(http.StatusOK)
}
