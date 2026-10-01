package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

func (h *Handler) handleExportCommand(ctx context.Context, month time.Time) error {
	data, filename, summary, err := h.csvExporter.Execute(ctx, month)
	if err != nil {
		h.logger.Error("erro ao gerar CSV", "error", err)
		h.sendText(ctx, "❌ Não consegui gerar a planilha. Tente novamente.")
		return err
	}

	if data == nil {
		h.sendText(ctx, fmt.Sprintf("📊 Sem lançamentos registrados em %s.", month.Format("01/2006")))
		return nil
	}

	caption := usecase.BuildExportCaption(month, summary)
	if _, err := h.messenger.SendDocument(ctx, h.owner, filename, data, caption); err != nil {
		h.logger.Error("erro ao enviar documento", "error", err)
	}
	return nil
}
