package domain

import "testing"

func TestCategoryLabel(t *testing.T) {
	cases := []struct {
		category Category
		expected string
	}{
		{CategoryFood, "Alimentação"},
		{CategoryTransport, "Transporte"},
		{CategoryHealth, "Saúde"},
		{CategoryEntertainment, "Lazer"},
		{CategoryShopping, "Compras"},
		{CategoryMarket, "Mercado"},
		{CategoryOther, "Outros"},
		{Category("UNKNOWN"), "Outros"},
	}
	for _, c := range cases {
		got := c.category.Label()
		if got != c.expected {
			t.Errorf("Category(%q).Label() = %q, esperado %q", c.category, got, c.expected)
		}
	}
}

func TestPaymentMethodLabel(t *testing.T) {
	cases := []struct {
		method   PaymentMethod
		expected string
	}{
		{PaymentMethodCash, "Dinheiro"},
		{PaymentMethodCreditCard, "Cartão de Crédito"},
		{PaymentMethodDebitCard, "Cartão de Débito"},
		{PaymentMethodPix, "Pix"},
		{PaymentMethodOther, "Outro"},
		{PaymentMethod("UNKNOWN"), "Outro"},
	}
	for _, c := range cases {
		got := c.method.Label()
		if got != c.expected {
			t.Errorf("PaymentMethod(%q).Label() = %q, esperado %q", c.method, got, c.expected)
		}
	}
}
