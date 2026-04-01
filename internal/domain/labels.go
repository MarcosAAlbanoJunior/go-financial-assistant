package domain

func (c Category) Label() string {
	switch c {
	case CategoryFood:
		return "Alimentação"
	case CategoryTransport:
		return "Transporte"
	case CategoryHealth:
		return "Saúde"
	case CategoryEntertainment:
		return "Lazer"
	case CategoryShopping:
		return "Compras"
	case CategoryMarket:
		return "Mercado"
	case CategoryInvestment:
		return "Investimento"
	default:
		return "Outros"
	}
}

func (p PaymentMethod) Label() string {
	switch p {
	case PaymentMethodCash:
		return "Dinheiro"
	case PaymentMethodCreditCard:
		return "Cartão de Crédito"
	case PaymentMethodDebitCard:
		return "Cartão de Débito"
	case PaymentMethodPix:
		return "Pix"
	default:
		return "Outro"
	}
}
