package pluggy

import (
	"context"
	"net/url"
	"time"
)

// SetOwnNames informa os seus nomes (como aparecem nas descrições do banco). Pix e transferências em que
// um deles aparece são entre contas suas e não viram lançamento. Pode ser chamado com o app rodando.
func (c *Client) SetOwnNames(names []string) { c.own.Set(names) }

func (c *Client) isOwnTransfer(description string) bool { return c.own.Match(description) }

// SetCredentials troca as credenciais da aplicação Pluggy com o app rodando; a apiKey em cache é descartada.
func (c *Client) SetCredentials(clientID, clientSecret string) {
	c.mu.Lock()
	c.clientID, c.clientSecret = clientID, clientSecret
	c.apiKey, c.apiKeyExp = "", time.Time{}
	c.mu.Unlock()
}

// ItemCheck é o resultado do teste de conexão de um item.
type ItemCheck struct {
	ItemID string
	Name   string // nome do banco, vazio se o conector não pôde ser lido
	OK     bool
}

// Check autentica e confere cada item. Erro só se a autenticação falhar; item inválido volta com OK falso.
func (c *Client) Check(ctx context.Context, itemIDs []string) ([]ItemCheck, error) {
	if _, err := c.authenticate(ctx, true); err != nil {
		return nil, err
	}
	out := make([]ItemCheck, 0, len(itemIDs))
	for _, id := range itemIDs {
		var res struct {
			Connector struct {
				Name string `json:"name"`
			} `json:"connector"`
		}
		err := c.get(ctx, "/items/"+url.PathEscape(id), &res)
		out = append(out, ItemCheck{ItemID: id, Name: res.Connector.Name, OK: err == nil})
	}
	return out, nil
}
