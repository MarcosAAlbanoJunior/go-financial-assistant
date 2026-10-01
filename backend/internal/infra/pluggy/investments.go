package pluggy

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

const maxInvestmentNameLen = 120

// investment lê só o que o app guarda; name é o produto (ex.: "CDB ..."), enquanto
// code, number, owner e issuerCNPJ (dados pessoais, segundo o Pluggy) são ignorados.
type investment struct {
	ID      string  `json:"id"`
	Type    string  `json:"type"`
	Subtype string  `json:"subtype"`
	Name    string  `json:"name"`
	Balance float64 `json:"balance"`
	Amount  float64 `json:"amount"`
}

func (i investment) toExternal(itemID string) ports.ExternalInvestment {
	name := strings.Join(strings.Fields(i.Name), " ")
	if r := []rune(name); len(r) > maxInvestmentNameLen {
		name = string(r[:maxInvestmentNameLen])
	}
	if name == "" {
		name = "Investimento"
	}
	return ports.ExternalInvestment{
		ID: i.ID, ItemID: itemID, Type: i.Type, Subtype: i.Subtype, Name: name, Balance: i.Balance, Amount: i.Amount,
	}
}

// FetchInvestments implementa ports.OpenFinanceProvider.
func (c *Client) FetchInvestments(ctx context.Context, itemID string) ([]ports.ExternalInvestment, error) {
	var all []ports.ExternalInvestment
	for page := 1; page <= maxPages; page++ {
		var res struct {
			TotalPages int          `json:"totalPages"`
			Results    []investment `json:"results"`
		}
		q := url.Values{"itemId": {itemID}, "page": {strconv.Itoa(page)}, "pageSize": {"500"}}
		if err := c.get(ctx, "/investments?"+q.Encode(), &res); err != nil {
			return nil, err
		}
		for _, inv := range res.Results {
			all = append(all, inv.toExternal(itemID))
		}
		if page >= res.TotalPages {
			break
		}
	}
	return all, nil
}
