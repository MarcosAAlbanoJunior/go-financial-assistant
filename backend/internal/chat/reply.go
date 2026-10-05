package chat

import (
	"fmt"
	"strings"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/ledger"
)

func formatStatementSummary(output *ledger.StatementOutput) string {
	if output.Inserted == 0 && len(output.Pending) == 0 {
		return "📄 Nenhuma transação nova encontrada no extrato."
	}

	var sb strings.Builder
	sb.WriteString("📄 *Extrato processado!*\n")
	if output.Inserted > 0 {
		fmt.Fprintf(&sb, "✅ %d transação(ões) importada(s) automaticamente.\n", output.Inserted)
	}
	if len(output.Pending) > 0 {
		fmt.Fprintf(&sb, "⚠️ %d transação(ões) já existem no banco — vou perguntar uma a uma.", len(output.Pending))
	}
	return sb.String()
}

func formatConfirmationQuestion(tx ledger.PendingTransaction, current, total int) string {
	return fmt.Sprintf(
		"❓ Transação %d/%d\n📅 %s\n📝 %s\n💰 R$ %.2f\n🏷️ %s\n\nJá existe uma transação com esse valor nessa data. Deseja inserir mesmo assim?\nResponda *sim* ou *não*",
		current, total,
		tx.Date.Format("02/01/2006"),
		tx.Description,
		tx.Amount,
		tx.Category,
	)
}

func formatReply(output *ledger.ExpenseOutput) string {
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
		return fmt.Sprintf("✅ Recorrente cancelada!\n📝 %s", output.CancelledDescription)
	case "INCOME":
		return fmt.Sprintf("✅ Entrada registrada!\n💰 R$ %.2f\n📝 %s\n🏷️ %s\n💳 %s",
			output.Amount, output.Description, output.Category, output.Payment)
	case "INCOME_RECURRING":
		return fmt.Sprintf("✅ Entrada recorrente registrada!\n💰 R$ %.2f/mês\n📝 %s\n🏷️ %s\n💳 %s\n📅 Dia %d de cada mês",
			output.Amount, output.Description, output.Category, output.Payment, output.DayOfMonth)
	case "TRANSFER":
		return fmt.Sprintf("↔️ Transferência registrada!\n💰 R$ %.2f\n📝 %s\n💳 %s",
			output.Amount, output.Description, output.Payment)
	default:
		return fmt.Sprintf("✅ Despesa registrada!\n💰 R$ %.2f\n📝 %s\n🏷️ %s\n💳 %s",
			output.Amount, output.Description, output.Category, output.Payment)
	}
}

func formatQueryReply(output *ledger.ExpenseOutput) string {
	if output.QueryEmpty {
		return fmt.Sprintf("📊 Sem lançamentos registrados em %s.", output.QueryMonth)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "📊 Resumo de %s\n\n", output.QueryMonth)

	if len(output.QueryCategories) > 0 {
		fmt.Fprintf(&sb, "💸 Despesas: R$ %.2f\n", output.QueryTotal)
		for _, c := range output.QueryCategories {
			fmt.Fprintf(&sb, "  • %s: R$ %.2f\n", c.Category, c.Total)
		}
	}

	if output.QueryIncome > 0 {
		fmt.Fprintf(&sb, "\n💰 Entradas: R$ %.2f\n", output.QueryIncome)
		fmt.Fprintf(&sb, "📈 Resultado: R$ %.2f\n", output.QueryBalance)
	}

	if output.QueryApplied > 0 || output.QueryRedeemed > 0 {
		sb.WriteString("\n🏦 Investimentos no mês\n")
		fmt.Fprintf(&sb, "  ↓ Aplicado: R$ %.2f\n", output.QueryApplied)
		fmt.Fprintf(&sb, "  ↑ Resgatado: R$ %.2f\n", output.QueryRedeemed)
		fmt.Fprintf(&sb, "💵 Em conta: R$ %.2f\n", output.QueryInAccount)
	}

	return sb.String()
}
