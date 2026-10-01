// Package pluggy lê contas e transações do Open Finance pela API do Pluggy
// (https://docs.pluggy.ai), usando as conexões feitas no Meu Pluggy.
package pluggy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	apiBase          = "https://api.pluggy.ai"
	maxResponseBytes = 10 << 20
	// A apiKey vale 2 horas; renovamos antes para nunca mandar uma expirada.
	apiKeyTTL = 100 * time.Minute
	// Trava de segurança contra cursor que nunca termina.
	maxPages = 100
)

type Client struct {
	clientID     string
	clientSecret string
	baseURL      string
	http         *http.Client

	mu        sync.Mutex
	apiKey    string
	apiKeyExp time.Time
}

func NewClient(clientID, clientSecret string) *Client {
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		baseURL:      apiBase,
		http:         &http.Client{Timeout: 30 * time.Second},
	}
}

// authenticate devolve a apiKey em cache ou cria uma nova (POST /auth).
func (c *Client) authenticate(ctx context.Context, force bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !force && c.apiKey != "" && time.Now().Before(c.apiKeyExp) {
		return c.apiKey, nil
	}

	body, _ := json.Marshal(map[string]string{"clientId": c.clientID, "clientSecret": c.clientSecret})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth", bytes.NewReader(body))
	if err != nil {
		return "", errors.New("pluggy auth: erro ao criar request")
	}
	req.Header.Set("Content-Type", "application/json")

	var res struct {
		APIKey string `json:"apiKey"`
	}
	if err := c.send(req, "auth", &res); err != nil {
		return "", err
	}
	if res.APIKey == "" {
		return "", errors.New("pluggy auth: resposta sem apiKey")
	}

	c.apiKey, c.apiKeyExp = res.APIKey, time.Now().Add(apiKeyTTL)
	return c.apiKey, nil
}

// get faz GET autenticado em baseURL+pathAndQuery. Se a apiKey for recusada (401),
// renova uma vez e repete.
func (c *Client) get(ctx context.Context, pathAndQuery string, out any) error {
	for attempt := 0; ; attempt++ {
		key, err := c.authenticate(ctx, attempt > 0)
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+pathAndQuery, nil)
		if err != nil {
			return errors.New("pluggy: erro ao criar request")
		}
		req.Header.Set("X-API-KEY", key)

		err = c.send(req, strings.SplitN(pathAndQuery, "?", 2)[0], out)
		var apiErr *apiError
		if attempt == 0 && errors.As(err, &apiErr) && apiErr.status == http.StatusUnauthorized {
			continue
		}
		return err
	}
}

type apiError struct {
	op      string
	status  int
	message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("pluggy %s: HTTP %d: %s", e.op, e.status, e.message)
}

// send executa a requisição. Erros de rede são reduzidos ao erro interno (sem URL) e
// mensagens da API são truncadas, para que segredos nunca cheguem aos logs.
func (c *Client) send(req *http.Request, op string, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("pluggy %s: %w", op, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("pluggy %s: erro ao ler resposta: %w", op, err)
	}

	if resp.StatusCode != http.StatusOK {
		var e struct {
			Message string `json:"message"`
		}
		json.Unmarshal(raw, &e) //nolint:errcheck
		if len(e.Message) > 200 {
			e.Message = e.Message[:200]
		}
		return &apiError{op: op, status: resp.StatusCode, message: e.Message}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("pluggy %s: resposta inválida", op)
	}
	return nil
}
