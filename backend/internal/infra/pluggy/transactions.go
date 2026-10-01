package pluggy

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

const (
	accountTypeCredit = "CREDIT"
	transactionsPath  = "/v2/transactions"
)

var ErrNoAccounts = errors.New("item sem contas: confira o itemId e se a conexão está autorizada no Meu Pluggy")

type account struct {
	ID   string `json:"id"`
	Type string `json:"type"` // BANK ou CREDIT
}

type transaction struct {
	ID             string  `json:"id"`
	Date           string  `json:"date"`
	Description    string  `json:"description"`
	DescriptionRaw string  `json:"descriptionRaw"`
	Amount         float64 `json:"amount"`
	Category       string  `json:"category"`
	Type           string  `json:"type"`   // DEBIT (saída) ou CREDIT (entrada)
	Status         string  `json:"status"` // POSTED ou PENDING
	PaymentData    *struct {
		PaymentMethod string `json:"paymentMethod"`
	} `json:"paymentData"`
	CreditCardMetadata *struct {
		InstallmentNumber int `json:"installmentNumber"`
		TotalInstallments int `json:"totalInstallments"`
	} `json:"creditCardMetadata"`
}

// FetchTransactions implementa ports.OpenFinanceProvider.
func (c *Client) FetchTransactions(ctx context.Context, itemID string, from time.Time) ([]ports.ExternalTransaction, error) {
	accounts, err := c.accounts(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, ErrNoAccounts
	}

	var result []ports.ExternalTransaction
	for _, acc := range accounts {
		txs, err := c.transactions(ctx, acc.ID, from)
		if err != nil {
			return nil, err
		}
		for _, t := range txs {
			if ext, ok := toExternal(acc.Type, t); ok {
				result = append(result, ext)
			}
		}
	}
	return result, nil
}

func (c *Client) accounts(ctx context.Context, itemID string) ([]account, error) {
	var all []account
	for page := 1; page <= maxPages; page++ {
		var res struct {
			TotalPages int       `json:"totalPages"`
			Results    []account `json:"results"`
		}
		q := url.Values{"itemId": {itemID}, "page": {strconv.Itoa(page)}}
		if err := c.get(ctx, "/accounts?"+q.Encode(), &res); err != nil {
			return nil, err
		}
		all = append(all, res.Results...)
		if page >= res.TotalPages {
			break
		}
	}
	return all, nil
}

func (c *Client) transactions(ctx context.Context, accountID string, from time.Time) ([]transaction, error) {
	path := transactionsPath + "?" + url.Values{
		"accountId": {accountID},
		"dateFrom":  {from.Format("2006-01-02")},
	}.Encode()

	var all []transaction
	for page := 0; page < maxPages; page++ {
		var res struct {
			Results []transaction `json:"results"`
			Next    *string       `json:"next"`
		}
		if err := c.get(ctx, path, &res); err != nil {
			return nil, err
		}
		all = append(all, res.Results...)

		if res.Next == nil || *res.Next == "" {
			return all, nil
		}
		var err error
		if path, err = c.nextPath(*res.Next); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("pluggy transactions: mais de %d páginas, abortando", maxPages)
}

// nextPath transforma o cursor "next" no caminho da próxima página. O cursor já vem
// codificado e é usado exatamente como veio (recodificar quebra o base64). Como a
// requisição leva a apiKey, só aceitamos URLs do próprio Pluggy, sempre em /v2/transactions.
func (c *Client) nextPath(next string) (string, error) {
	switch {
	case strings.HasPrefix(next, "http"):
		u, err := url.Parse(next)
		base, _ := url.Parse(c.baseURL)
		if err != nil || u.Host != base.Host || !strings.HasPrefix(u.Path, transactionsPath) {
			return "", errors.New("pluggy transactions: cursor de paginação inesperado")
		}
		return u.RequestURI(), nil
	case strings.HasPrefix(next, "/"):
		if !strings.HasPrefix(next, transactionsPath) {
			return "", errors.New("pluggy transactions: cursor de paginação inesperado")
		}
		return next, nil
	case strings.HasPrefix(next, "?"):
		return transactionsPath + next, nil
	}
	return transactionsPath + "?" + next, nil
}
