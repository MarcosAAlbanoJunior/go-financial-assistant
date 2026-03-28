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
)

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
	return c.postAndExtractID(ctx, endpoint, body, "mensagem")
}

func (c *Client) SendDocument(ctx context.Context, to, filename, base64Data, caption string) (string, error) {
	if idx := strings.Index(to, "@"); idx != -1 {
		to = to[:idx]
	}

	body, err := json.Marshal(map[string]any{
		"number":    to,
		"mediatype": "document",
		"fileName":  filename,
		"media":     base64Data,
		"caption":   caption,
	})
	if err != nil {
		return "", fmt.Errorf("erro ao serializar documento: %w", err)
	}

	endpoint := fmt.Sprintf("%s/message/sendMedia/%s", c.baseURL, url.PathEscape(c.instance))
	return c.postAndExtractID(ctx, endpoint, body, "documento")
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

func (c *Client) postAndExtractID(ctx context.Context, endpoint string, body []byte, kind string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("erro ao criar request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("erro ao enviar %s: %w", kind, err)
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
