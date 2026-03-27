package evolution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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

type sendTextResponse struct {
	Key struct {
		ID string `json:"id"`
	} `json:"key"`
}

type fetchBase64Response struct {
	Base64 string `json:"base64"`
}

func (c *Client) SendText(ctx context.Context, to string, text string) (string, error) {
	if idx := strings.Index(to, "@"); idx != -1 {
		to = to[:idx]
	}

	body, err := json.Marshal(map[string]string{
		"number": to,
		"text":   text,
	})
	if err != nil {
		return "", fmt.Errorf("erro ao serializar mensagem: %w", err)
	}

	endpoint := fmt.Sprintf("%s/message/sendText/%s", c.baseURL, url.PathEscape(c.instance))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("erro ao criar request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("erro ao enviar mensagem: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("erro ao ler resposta: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("evolution API retornou status %d: %s", resp.StatusCode, string(respBody))
	}

	var result sendTextResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("erro ao deserializar resposta: %w", err)
	}

	return result.Key.ID, nil
}

func (c *Client) FetchImageBase64(ctx context.Context, remoteJid string, fromMe bool, messageID string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"key": map[string]any{
				"remoteJid": remoteJid,
				"fromMe":    fromMe,
				"id":        messageID,
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("erro ao serializar request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/chat/getBase64FromMediaMessage/%s", c.baseURL, url.PathEscape(c.instance))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("erro ao criar request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("erro ao buscar base64: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("erro ao ler resposta: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("evolution API retornou status %d: %s", resp.StatusCode, string(respBody))
	}

	var result fetchBase64Response
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("erro ao deserializar resposta: %w", err)
	}

	return result.Base64, nil
}
