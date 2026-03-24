package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

func (h *webhookHandler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *webhookHandler) writeError(w http.ResponseWriter, msg string, status int) {
	h.writeJSON(w, status, map[string]string{"error": msg})
}

func (h *webhookHandler) handleError(w http.ResponseWriter, err error) {
	h.logger.Error("erro ao processar webhook", "error", err)

	switch {
	case errors.Is(err, domain.ErrInvalidAmount):
		h.writeError(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrInvalidPaymentMethod):
		h.writeError(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, errUnsupportedMessage):
		h.writeError(w, "tipo de mensagem não suportado", http.StatusBadRequest)
	case errors.Is(err, errInvalidImage):
		h.writeError(w, "imagem inválida ou corrompida", http.StatusBadRequest)
	default:
		h.writeError(w, "erro interno", http.StatusInternalServerError)
	}
}

func formatReply(output *usecase.ExpenseOutput) string {
	switch output.Type {
	case "INSTALLMENT":
		return fmt.Sprintf(
			"✅ Compra parcelada registrada!\n💰 Total: R$ %.2f\n📅 %dx de R$ %.2f\n📝 %s\n🏷️ %s\n💳 %s",
			output.Amount, output.TotalInstallments, output.InstallmentAmount,
			output.Description, output.Category, output.Payment,
		)
	case "RECURRING":
		return fmt.Sprintf(
			"✅ Despesa recorrente registrada!\n💰 R$ %.2f/mês\n📝 %s\n🏷️ %s\n💳 %s\n📅 Dia %d de cada mês",
			output.Amount, output.Description, output.Category, output.Payment, output.DayOfMonth,
		)
	case "CANCEL_RECURRING":
		return fmt.Sprintf("✅ Despesa recorrente cancelada!\n📝 %s", output.CancelledDescription)
	default:
		return fmt.Sprintf("✅ Despesa registrada!\n💰 R$ %.2f\n📝 %s\n🏷️ %s\n💳 %s",
			output.Amount, output.Description, output.Category, output.Payment)
	}
}

var (
	errUnsupportedMessage = errors.New("tipo de mensagem não suportado")
	errInvalidImage       = errors.New("imagem inválida")
)
