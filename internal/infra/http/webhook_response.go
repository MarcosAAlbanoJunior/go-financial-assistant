package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

func formatStatementSummary(output *usecase.StatementOutput) string {
	if output.Inserted == 0 && len(output.Pending) == 0 {
		return "📄 Nenhuma despesa nova encontrada no extrato."
	}

	var sb strings.Builder
	sb.WriteString("📄 *Extrato processado!*\n")
	if output.Inserted > 0 {
		sb.WriteString(fmt.Sprintf("✅ %d despesa(s) importada(s) automaticamente.\n", output.Inserted))
	}
	if len(output.Pending) > 0 {
		sb.WriteString(fmt.Sprintf("⚠️ %d transação(ões) já existem no banco — vou perguntar uma a uma.", len(output.Pending)))
	}
	return sb.String()
}

func formatConfirmationQuestion(tx usecase.PendingTransaction, current, total int) string {
	return fmt.Sprintf(
		"❓ Transação %d/%d\n📅 %s\n📝 %s\n💰 R$ %.2f\n🏷️ %s\n\nJá existe uma transação com esse valor nessa data. Deseja inserir mesmo assim?\nResponda *sim* ou *não*",
		current, total,
		tx.Date.Format("02/01/2006"),
		tx.Description,
		tx.Amount,
		tx.Category,
	)
}

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
	case "QUERY":
		return formatQueryReply(output)
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

func formatQueryReply(output *usecase.ExpenseOutput) string {
	if output.QueryEmpty {
		return fmt.Sprintf("📊 Sem despesas registradas em %s.", output.QueryMonth)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 Despesas de %s\n", output.QueryMonth))
	sb.WriteString(fmt.Sprintf("💰 Total: R$ %.2f\n\n", output.QueryTotal))
	for _, c := range output.QueryCategories {
		sb.WriteString(fmt.Sprintf("  • %s: R$ %.2f\n", c.Category, c.Total))
	}
	return sb.String()
}

var (
	errUnsupportedMessage = errors.New("tipo de mensagem não suportado")
	errInvalidImage       = errors.New("imagem inválida")
)
