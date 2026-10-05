// Package telegram implementa o canal Telegram sobre a Bot API oficial
// (https://core.telegram.org/bots/api), usando apenas a biblioteca padrão.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	apiBase = "https://api.telegram.org"
	// A Bot API só permite baixar arquivos de até 20 MB.
	maxDownloadBytes = 20 << 20
	maxResponseBytes = 10 << 20
)

type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

func NewClient(token string) *Client {
	return &Client{token: token, baseURL: apiBase, http: &http.Client{Timeout: 60 * time.Second}}
}

type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
}

// do executa a requisição e devolve o corpo bruto. O token faz parte da URL da API,
// então nunca devolvemos o *url.Error original: ele incluiria a URL (e o token) em logs.
func (c *Client) do(req *http.Request, op string) ([]byte, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, fmt.Errorf("telegram %s: %w", op, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("telegram %s: erro ao ler resposta: %w", op, err)
	}
	return body, nil
}

// APIError é uma recusa da Bot API (token inválido = 401/404; outro programa lendo as mensagens = 409).
type APIError struct {
	Method      string
	Description string
	Code        int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram %s: %s (código %d)", e.Method, e.Description, e.Code)
}

func (c *Client) call(ctx context.Context, method, contentType string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/bot%s/%s", c.baseURL, c.token, method), body)
	if err != nil {
		return fmt.Errorf("telegram %s: erro ao criar request", method)
	}
	req.Header.Set("Content-Type", contentType)

	raw, err := c.do(req, method)
	if err != nil {
		return err
	}

	var res apiResponse
	if err := json.Unmarshal(raw, &res); err != nil {
		return fmt.Errorf("telegram %s: resposta inválida", method)
	}
	if !res.OK {
		return &APIError{Method: method, Description: res.Description, Code: res.ErrorCode}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(res.Result, out)
}

func (c *Client) callJSON(ctx context.Context, method string, params, out any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("telegram %s: erro ao serializar params: %w", method, err)
	}
	return c.call(ctx, method, "application/json", bytes.NewReader(body), out)
}

// GetMe valida o token e devolve o @username do bot.
func (c *Client) GetMe(ctx context.Context) (string, error) {
	var me struct {
		Username string `json:"username"`
	}
	if err := c.callJSON(ctx, "getMe", struct{}{}, &me); err != nil {
		return "", err
	}
	return me.Username, nil
}

// SendText implementa ports.Messenger. O Telegram não devolve eco das mensagens do
// próprio bot, então o messageID é sempre "".
func (c *Client) SendText(ctx context.Context, to, text string) (string, error) {
	return "", c.callJSON(ctx, "sendMessage", map[string]string{
		"chat_id":    to,
		"text":       toHTML(text, maxTextRunes),
		"parse_mode": "HTML",
	}, nil)
}

func (c *Client) SendDocument(ctx context.Context, to, filename string, data []byte, caption string) (string, error) {
	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	fields := [][2]string{{"chat_id", to}, {"caption", toHTML(caption, maxCaptionRunes)}, {"parse_mode", "HTML"}}
	for _, f := range fields {
		if err := form.WriteField(f[0], f[1]); err != nil {
			return "", fmt.Errorf("telegram sendDocument: erro ao montar upload: %w", err)
		}
	}
	part, err := form.CreateFormFile("document", filename)
	if err != nil {
		return "", fmt.Errorf("telegram sendDocument: erro ao montar upload: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return "", fmt.Errorf("telegram sendDocument: erro ao montar upload: %w", err)
	}
	if err := form.Close(); err != nil {
		return "", fmt.Errorf("telegram sendDocument: erro ao montar upload: %w", err)
	}
	return "", c.call(ctx, "sendDocument", form.FormDataContentType(), &buf, nil)
}

// Download baixa um arquivo enviado ao bot (getFile + download).
func (c *Client) Download(ctx context.Context, fileID string) ([]byte, error) {
	var file struct {
		FilePath string `json:"file_path"`
		FileSize int64  `json:"file_size"`
	}
	if err := c.callJSON(ctx, "getFile", map[string]string{"file_id": fileID}, &file); err != nil {
		return nil, err
	}
	if file.FilePath == "" || file.FileSize > maxDownloadBytes {
		return nil, errors.New("telegram: arquivo indisponível ou maior que 20 MB")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/file/bot%s/%s", c.baseURL, c.token, file.FilePath), nil)
	if err != nil {
		return nil, errors.New("telegram download: erro ao criar request")
	}
	data, err := c.do(req, "download")
	if err != nil {
		return nil, err
	}
	if len(data) > maxDownloadBytes {
		return nil, errors.New("telegram: arquivo maior que 20 MB")
	}
	return data, nil
}

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

type message struct {
	Text     string      `json:"text"`
	Caption  string      `json:"caption"`
	Chat     chatInfo    `json:"chat"`
	From     *user       `json:"from"`
	Photo    []photoSize `json:"photo"`
	Document *document   `json:"document"`
}

type chatInfo struct {
	Type string `json:"type"`
}

type user struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type photoSize struct {
	FileID string `json:"file_id"`
}

type document struct {
	FileID   string `json:"file_id"`
	MimeType string `json:"mime_type"`
}

// getUpdates faz long polling. Falha com 409 se o bot tiver webhook configurado.
func (c *Client) getUpdates(ctx context.Context, offset int64, timeoutSec int) ([]update, error) {
	var updates []update
	err := c.callJSON(ctx, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         timeoutSec,
		"allowed_updates": []string{"message"},
	}, &updates)
	return updates, err
}

// Sender é quem mandou uma mensagem privada ao bot.
type Sender struct {
	ID       int64
	Name     string // nome e sobrenome como estão no Telegram
	Username string // sem @; pode ser vazio
}

// LatestOffset devolve o offset logo depois da última mensagem já recebida pelo bot, para o setup só considerar o que
// chegar a partir de agora.
func (c *Client) LatestOffset(ctx context.Context) (int64, error) {
	updates, err := c.getUpdates(ctx, -1, 0)
	if err != nil || len(updates) == 0 {
		return 0, err
	}
	return updates[len(updates)-1].UpdateID + 1, nil
}

// FirstPrivate procura, a partir de offset e sem esperar, a primeira mensagem de conversa privada (mensagens de grupo são
// ignoradas). Devolve quem mandou (nil se ninguém) e o offset seguinte ao último update lido. Usado só no setup, antes de
// o bot subir: os dois leem getUpdates e não podem rodar juntos.
func (c *Client) FirstPrivate(ctx context.Context, offset int64) (*Sender, int64, error) {
	updates, err := c.getUpdates(ctx, offset, 0)
	if err != nil {
		return nil, offset, err
	}
	next := offset
	for _, u := range updates {
		next = u.UpdateID + 1
		if m := u.Message; m != nil && m.Chat.Type == "private" && m.From != nil {
			name := strings.TrimSpace(m.From.FirstName + " " + m.From.LastName)
			return &Sender{ID: m.From.ID, Name: name, Username: m.From.Username}, next, nil
		}
	}
	return nil, next, nil
}
