package evolution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL    string
	instance   string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL, instance, apiKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		instance:   instance,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// response types compartilhados entre os arquivos do pacote
type sendTextResponse struct {
	Key struct {
		ID string `json:"id"`
	} `json:"key"`
}

type fetchBase64Response struct {
	Base64 string `json:"base64"`
}

type connectionStateResponse struct {
	Instance struct {
		State string `json:"state"`
	} `json:"instance"`
}

type connectResponse struct {
	Code string `json:"code"`
}

func (c *Client) EnsureInstance(ctx context.Context, ownerPhone string) (bool, error) {
	body, err := json.Marshal(map[string]any{
		"instanceName": c.instance,
		"integration":  "WHATSAPP-BAILEYS",
		"number":       ownerPhone,
		"qrcode":       true,
		"token":        c.apiKey,
	})
	if err != nil {
		return false, fmt.Errorf("erro ao serializar request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/instance/create", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("erro ao criar request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("erro ao criar instância: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return false, nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("erro ao ler resposta: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return false, fmt.Errorf("evolution API retornou status %d: %s", resp.StatusCode, string(respBody))
	}

	return true, nil
}

func (c *Client) FetchConnectionState(ctx context.Context) (string, error) {
	endpoint := fmt.Sprintf("%s/instance/connectionState/%s", c.baseURL, url.PathEscape(c.instance))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("erro ao criar request: %w", err)
	}

	req.Header.Set("apikey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("erro ao buscar estado da conexão: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("erro ao ler resposta: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("evolution API retornou status %d: %s", resp.StatusCode, string(respBody))
	}

	var result connectionStateResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("erro ao deserializar resposta: %w", err)
	}

	return result.Instance.State, nil
}

func (c *Client) FetchConnectCode(ctx context.Context) (string, error) {
	endpoint := fmt.Sprintf("%s/instance/connect/%s", c.baseURL, url.PathEscape(c.instance))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("erro ao criar request: %w", err)
	}

	req.Header.Set("apikey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("erro ao buscar QR code: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("erro ao ler resposta: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("evolution API retornou status %d: %s", resp.StatusCode, string(respBody))
	}

	var result connectResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("erro ao deserializar resposta: %w", err)
	}

	return result.Code, nil
}
