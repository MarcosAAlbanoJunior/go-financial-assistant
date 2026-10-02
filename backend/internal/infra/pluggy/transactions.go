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

// account lê só o que o app guarda; taxNumber, owner e o número completo são ignorados de propósito.
type account struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"` // BANK ou CREDIT
	Name       string  `json:"name"`
	Number     string  `json:"number"` // BANK: "0001/12345-0"; CREDIT: "xxxx8670"
	Balance    float64 `json:"balance"`
	CreditData *struct {
		CreditLimit          *float64 `json:"creditLimit"`
		AvailableCreditLimit *float64 `json:"availableCreditLimit"`
		Brand                string   `json:"brand"`
		BalanceCloseDate     string   `json:"balanceCloseDate"`
		BalanceDueDate       string   `json:"balanceDueDate"`
		MinimumPayment       *float64 `json:"minimumPayment"`
	} `json:"creditData"`
	BankData *struct {
		AutomaticallyInvestedBalance *float64 `json:"automaticallyInvestedBalance"`
	} `json:"bankData"`
}

func (a account) toExternal(itemID string) ports.ExternalAccount {
	ext := ports.ExternalAccount{
		ID: a.ID, ItemID: itemID, Type: a.Type, Balance: a.Balance,
		Name:  strings.Join(strings.Fields(a.Name), " "),
		Last4: last4(a.Number),
	}
	if a.CreditData != nil {
		cd := a.CreditData
		ext.CreditLimit, ext.AvailableCreditLimit = cd.CreditLimit, cd.AvailableCreditLimit
		ext.Brand = strings.ToUpper(strings.Join(strings.Fields(cd.Brand), " "))
		ext.CloseDate, ext.DueDate = parseDay(cd.BalanceCloseDate), parseDay(cd.BalanceDueDate)
		ext.MinimumPayment = cd.MinimumPayment
	}
	if a.BankData != nil {
		ext.AutoInvested = a.BankData.AutomaticallyInvestedBalance
	}
	return ext
}

// parseDay lê uma data do Pluggy ("2026-10-10" ou ISO completo) como o dia civil, em UTC; nil se vazia ou inválida.
func parseDay(s string) *time.Time {
	if len(s) < 10 {
		return nil
	}
	t, err := time.Parse("2006-01-02", s[:10])
	if err != nil {
		return nil
	}
	return &t
}

// last4 devolve os 4 últimos dígitos do número da conta ou do cartão.
func last4(number string) string {
	var digits []rune
	for _, r := range number {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	if len(digits) > 4 {
		digits = digits[len(digits)-4:]
	}
	return string(digits)
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

// FetchItem implementa ports.OpenFinanceProvider.
func (c *Client) FetchItem(ctx context.Context, itemID string, from time.Time) (ports.ItemData, error) {
	accounts, err := c.accounts(ctx, itemID)
	if err != nil {
		return ports.ItemData{}, err
	}
	if len(accounts) == 0 {
		return ports.ItemData{}, ErrNoAccounts
	}

	var data ports.ItemData
	// O conector só enriquece a tela: se não puder ser lido, as contas e as transações seguem normalmente.
	data.Institution = c.institution(ctx, itemID)
	for _, acc := range accounts {
		data.Accounts = append(data.Accounts, acc.toExternal(itemID))
		txs, err := c.transactions(ctx, acc.ID, from)
		if err != nil {
			return ports.ItemData{}, err
		}
		for _, t := range txs {
			if ext, ok := toExternal(acc.Type, t); ok {
				ext.AccountID = acc.ID
				data.Transactions = append(data.Transactions, ext)
			}
		}
	}
	return data, nil
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
