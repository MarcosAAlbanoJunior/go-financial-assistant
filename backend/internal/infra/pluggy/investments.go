package pluggy

import (
	"context"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

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
		var (
			res struct {
				TotalPages int          `json:"totalPages"`
				Results    []investment `json:"results"`
			}
			err error
		)
		q := url.Values{"itemId": {itemID}, "page": {strconv.Itoa(page)}, "pageSize": {"500"}}
		if err = c.get(ctx, "/investments?"+q.Encode(), &res); err != nil {
			return nil, err
		}
		for _, inv := range res.Results {
			ext := inv.toExternal(itemID)
			// Uma posição sem movimentações legíveis continua valendo: só fica sem estimativa.
			if ext.Movements, err = c.investmentMovements(ctx, inv.ID); err != nil {
				ext.MovementsFailed = true
			}
			all = append(all, ext)
		}
		if page >= res.TotalPages {
			break
		}
	}
	return all, nil
}

type movement struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"`         // BUY, SELL, TAX, TRANSFER...
	MovementType string  `json:"movementType"` // CREDIT (entra na posição) ou DEBIT (sai)
	Amount       float64 `json:"amount"`       // sem sinal
	Date         string  `json:"date"`
	TradeDate    string  `json:"tradeDate"`
}

// toExternal devolve a movimentação com sinal. O sentido vem de movementType; sem ele, de type
// (BUY entra, SELL e TAX saem). Qualquer outra coisa é descartada, pois não dá para saber o efeito.
func (m movement) toExternal() (ports.ExternalMovement, bool) {
	var sign float64
	switch {
	case m.MovementType == "CREDIT":
		sign = 1
	case m.MovementType == "DEBIT":
		sign = -1
	case m.Type == "BUY":
		sign = 1
	case m.Type == "SELL" || m.Type == "TAX":
		sign = -1
	default:
		return ports.ExternalMovement{}, false
	}
	raw := m.Date
	if raw == "" {
		raw = m.TradeDate
	}
	day, err := time.Parse(time.RFC3339, raw)
	amount := math.Abs(m.Amount)
	if err != nil || m.ID == "" || amount == 0 {
		return ports.ExternalMovement{}, false
	}
	return ports.ExternalMovement{ID: m.ID, Day: day.UTC().Truncate(24 * time.Hour), Amount: sign * amount}, true
}

func (c *Client) investmentMovements(ctx context.Context, investmentID string) ([]ports.ExternalMovement, error) {
	var all []ports.ExternalMovement
	for page := 1; page <= maxPages; page++ {
		var res struct {
			TotalPages int        `json:"totalPages"`
			Results    []movement `json:"results"`
		}
		q := url.Values{"page": {strconv.Itoa(page)}, "pageSize": {"500"}}
		if err := c.get(ctx, "/investments/"+url.PathEscape(investmentID)+"/transactions?"+q.Encode(), &res); err != nil {
			return nil, err
		}
		for _, m := range res.Results {
			if ext, ok := m.toExternal(); ok {
				all = append(all, ext)
			}
		}
		if page >= res.TotalPages {
			break
		}
	}
	return all, nil
}
