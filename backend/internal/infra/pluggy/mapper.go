package pluggy

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// A categoria do Pluggy chega como texto em inglês (ex.: "Groceries", "Same person
// transfer - PIX"). Casamos por trechos, em ordem; a primeira regra que bater vence.
// Categorias novas ou não mapeadas caem em OTHER sem quebrar nada.
var (
	// Não são gasto nem renda: a fatura já é contada pelas compras do cartão, e
	// transferência entre contas do mesmo titular só move dinheiro de lugar.
	ignoredCategories = []string{"credit card payment", "same person transfer"}

	// Aplicação (saída) e resgate (entrada) de investimentos viram TRANSFER.
	investmentCategories = []string{"investment", "fixed income", "variable income", "mutual fund"}

	// Aplicação e resgate automáticos do Itaú ("APLIC AUT MAIS") só varrem o saldo da conta
	// para um CDB e de volta: não são aportes seus e inflariam o fluxo de investimentos.
	// Os rendimentos pagos por essa aplicação são renda e continuam entrando.
	autoSweepDescriptions = []string{"aplic aut mais"}

	// Quando o Pluggy não classifica a despesa (OTHER), a descrição decide. Vale a primeira regra
	// que bater; nomes de pessoas e o que não for óbvio ficam em OTHER de propósito.
	descriptionRules = []struct {
		keywords []string
		category domain.Category
	}{
		{[]string{"ifood", "99 food", "rappi", "uber eats"}, domain.CategoryFood},
		{[]string{"supermercado", "atacadao", "assai"}, domain.CategoryMarket},
		{[]string{"auto posto", " posto ", "combustivel", "uber", "99 pop", "sem parar"}, domain.CategoryTransport},
		{[]string{"rd saude", "drogaria", "drogasil", "farmacia", "pague menos", "santa casa", "clinica", "clínica", "hospital"}, domain.CategoryHealth},
		{[]string{"kalunga", "magalupay", "magazine luiza", "mercadolivre", "mercado livre"}, domain.CategoryShopping},
		{[]string{"anthropic", "hostinger", "ionos", "spotify", "netflix", "apple.com"}, domain.CategoryEntertainment},
	}

	expenseCategories = []struct {
		keywords []string
		category domain.Category
	}{
		{[]string{"groceries", "supermarket"}, domain.CategoryMarket},
		{[]string{"food", "restaurant", "eating", "delivery", "bakery"}, domain.CategoryFood},
		{[]string{"transport", "automotive", "fuel", "taxi", "ride", "parking", "vehicle"}, domain.CategoryTransport},
		{[]string{"health", "pharmacy", "medical", "dental"}, domain.CategoryHealth},
		{[]string{"leisure", "entertainment", "digital services", "travel", "streaming", "gaming"}, domain.CategoryEntertainment},
		{[]string{"shopping", "clothing", "electronics"}, domain.CategoryShopping},
	}
)

func containsAny(s string, keywords []string) bool {
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// toExternal converte a transação do Pluggy para o vocabulário do app. Devolve false
// quando ela não deve virar lançamento. O sentido (entrada/saída) vem do campo "type",
// não do sinal do valor, porque o sinal é invertido nos cartões de crédito.
func toExternal(accountType string, t transaction) (ports.ExternalTransaction, bool) {
	date, err := time.Parse(time.RFC3339, t.Date)
	amount := math.Abs(t.Amount)
	if err != nil || amount == 0 || t.ID == "" {
		return ports.ExternalTransaction{}, false
	}
	date = date.UTC().Truncate(24 * time.Hour)

	category := strings.ToLower(t.Category)
	isCard := accountType == accountTypeCredit
	inflow := t.Type == "CREDIT"

	// Em cartão, crédito é pagamento de fatura ou estorno: não é renda.
	if containsAny(category, ignoredCategories) || (isCard && inflow) {
		return ports.ExternalTransaction{}, false
	}

	description := t.Description
	if description == "" {
		description = t.DescriptionRaw
	}
	if m := t.CreditCardMetadata; m != nil && m.TotalInstallments > 1 {
		description = fmt.Sprintf("%s (%d/%d)", description, m.InstallmentNumber, m.TotalInstallments)
	}

	ext := ports.ExternalTransaction{
		ID:            t.ID,
		Date:          date,
		Description:   description,
		RawInput:      fmt.Sprintf("[open finance: %s | %s]", t.Category, t.DescriptionRaw),
		Amount:        amount,
		Category:      domain.CategoryOther,
		PaymentMethod: paymentMethod(isCard, t),
		Pending:       t.Status == "PENDING",
	}

	switch {
	case containsAny(category, investmentCategories):
		if containsAny(strings.ToLower(description), autoSweepDescriptions) {
			return ports.ExternalTransaction{}, false
		}
		ext.Kind, ext.Category = domain.KindTransfer, domain.CategoryInvestment
		ext.Direction = domain.TransferDirectionOut
		if inflow {
			ext.Direction = domain.TransferDirectionIn
		}
	case inflow:
		ext.Kind = domain.KindIncome
		if strings.Contains(category, "salary") {
			ext.Category = domain.CategorySalary
		}
	default:
		ext.Kind = domain.KindExpense
		for _, rule := range expenseCategories {
			if containsAny(category, rule.keywords) {
				ext.Category = rule.category
				break
			}
		}
		if ext.Category == domain.CategoryOther {
			ext.Category = categoryFromDescription(description + " " + t.DescriptionRaw)
		}
	}
	return ext, true
}

// categoryFromDescription aplica descriptionRules; sem correspondência devolve OTHER.
func categoryFromDescription(text string) domain.Category {
	text = " " + strings.ToLower(text) + " "
	for _, rule := range descriptionRules {
		if containsAny(text, rule.keywords) {
			return rule.category
		}
	}
	return domain.CategoryOther
}

func paymentMethod(isCard bool, t transaction) domain.PaymentMethod {
	if isCard {
		return domain.PaymentMethodCreditCard
	}
	if t.PaymentData != nil && strings.EqualFold(t.PaymentData.PaymentMethod, "PIX") {
		return domain.PaymentMethodPix
	}
	return domain.PaymentMethodOther
}
