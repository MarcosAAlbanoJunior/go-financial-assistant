package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"google.golang.org/genai"
)

const modelName = "gemini-2.5-flash-lite"

type Client struct {
	client *genai.Client
	config *genai.GenerateContentConfig
}

func NewClient(ctx context.Context, apiKey string) (*Client, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cliente gemini: %w", err)
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: systemPrompt},
			},
		},
		ResponseMIMEType: "application/json",
	}

	return &Client{client: client, config: config}, nil
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) AnalyzeText(ctx context.Context, text string) (*ports.ExpenseAnalysis, error) {
	prompt := fmt.Sprintf("Analise esta entrada financeira e extraia as informações:\n\n%s", text)

	resp, err := c.client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), c.config)
	if err != nil {
		return nil, fmt.Errorf("erro ao chamar gemini: %w", err)
	}

	return parseResponse(resp)
}

func (c *Client) AnalyzeImage(ctx context.Context, imageData []byte, mimeType string) (*ports.ExpenseAnalysis, error) {
	contents := []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				{InlineData: &genai.Blob{MIMEType: mimeType, Data: imageData}},
				{Text: "Analise esta nota fiscal ou recibo e extraia as informações da despesa."},
			},
		},
	}

	resp, err := c.client.Models.GenerateContent(ctx, modelName, contents, c.config)
	if err != nil {
		return nil, fmt.Errorf("erro ao analisar imagem com gemini: %w", err)
	}

	return parseResponse(resp)
}

func (c *Client) AnalyzeDocument(ctx context.Context, data []byte, mimeType string) (*ports.StatementAnalysis, error) {
	statementConfig := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: statementPrompt},
			},
		},
		ResponseMIMEType: "application/json",
	}

	contents := []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				{InlineData: &genai.Blob{MIMEType: mimeType, Data: data}},
				{Text: "Extraia todas as transações deste extrato bancário."},
			},
		},
	}

	resp, err := c.client.Models.GenerateContent(ctx, modelName, contents, statementConfig)
	if err != nil {
		return nil, fmt.Errorf("erro ao analisar extrato com gemini: %w", err)
	}

	return parseStatementResponse(resp)
}

func parseResponse(resp *genai.GenerateContentResponse) (*ports.ExpenseAnalysis, error) {
	if resp == nil || len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("gemini retornou resposta vazia")
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini retornou conteúdo vazio")
	}

	rawJSON := candidate.Content.Parts[0].Text

	var geminiResp geminiResponse
	if err := json.Unmarshal([]byte(rawJSON), &geminiResp); err != nil {
		return nil, fmt.Errorf("erro ao deserializar resposta gemini: %w", err)
	}

	return geminiResp.toAnalysis(rawJSON), nil
}

type geminiInstallments struct {
	Total                int     `json:"total"`
	AmountPerInstallment float64 `json:"amount_per_installment"`
}

type geminiRecurring struct {
	DayOfMonth int `json:"day_of_month"`
}

type geminiCancelRecurring struct {
	Description string `json:"description"`
}

type geminiQuery struct {
	Month *int `json:"month"`
	Year  *int `json:"year"`
}

type geminiResponse struct {
	Type            string                 `json:"type"`
	Amount          *float64               `json:"amount"`
	Description     *string                `json:"description"`
	Category        *string                `json:"category"`
	PaymentMethod   *string                `json:"payment_method"`
	Confidence      float64                `json:"confidence"`
	Installments    *geminiInstallments    `json:"installments"`
	Recurring       *geminiRecurring       `json:"recurring"`
	CancelRecurring *geminiCancelRecurring `json:"cancel_recurring"`
	Query           *geminiQuery           `json:"query"`
	Export          *geminiQuery           `json:"export"`
}

func (g *geminiResponse) toAnalysis(rawJSON string) *ports.ExpenseAnalysis {
	analysis := &ports.ExpenseAnalysis{
		Amount:        g.Amount,
		Description:   g.Description,
		Category:      g.Category,
		PaymentMethod: g.PaymentMethod,
		Confidence:    g.Confidence,
		RawResponse:   rawJSON,
		Type:          toExpenseType(g.Type),
	}

	if g.Installments != nil {
		analysis.Installments = &ports.InstallmentInfo{
			Total:                g.Installments.Total,
			AmountPerInstallment: g.Installments.AmountPerInstallment,
		}
	}

	if g.Recurring != nil {
		analysis.RecurringInfo = &ports.RecurringInfo{
			DayOfMonth: g.Recurring.DayOfMonth,
		}
	}

	if g.CancelRecurring != nil {
		analysis.CancelInfo = &ports.CancelInfo{
			Description: g.CancelRecurring.Description,
		}
	}

	if g.Query != nil {
		info := &ports.QueryInfo{}
		if g.Query.Month != nil {
			info.Month = *g.Query.Month
		}
		if g.Query.Year != nil {
			info.Year = *g.Query.Year
		}
		analysis.QueryInfo = info
	}

	if g.Export != nil {
		info := &ports.QueryInfo{}
		if g.Export.Month != nil {
			info.Month = *g.Export.Month
		}
		if g.Export.Year != nil {
			info.Year = *g.Export.Year
		}
		analysis.ExportInfo = info
	}

	return analysis
}

type geminiStatementTransaction struct {
	Date           string  `json:"date"`
	RawDescription string  `json:"raw_description"`
	Description    string  `json:"description"`
	Amount         float64 `json:"amount"`
	Kind           string  `json:"kind"`
	Category       string  `json:"category"`
	PaymentMethod  string  `json:"payment_method"`
}

type geminiStatementResponse struct {
	Transactions []geminiStatementTransaction `json:"transactions"`
}

func parseStatementResponse(resp *genai.GenerateContentResponse) (*ports.StatementAnalysis, error) {
	if resp == nil || len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("gemini retornou resposta vazia")
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini retornou conteúdo vazio")
	}

	var raw geminiStatementResponse
	if err := json.Unmarshal([]byte(candidate.Content.Parts[0].Text), &raw); err != nil {
		return nil, fmt.Errorf("erro ao deserializar extrato gemini: %w", err)
	}

	analysis := &ports.StatementAnalysis{
		Transactions: make([]ports.StatementTransaction, 0, len(raw.Transactions)),
	}

	for _, t := range raw.Transactions {
		parsed, err := parseStatementDate(t.Date)
		if err != nil {
			continue // ignora linha com data inválida
		}
		kind := t.Kind
		if kind != "INCOME" {
			kind = "EXPENSE"
		}
		analysis.Transactions = append(analysis.Transactions, ports.StatementTransaction{
			Date:           parsed,
			RawDescription: t.RawDescription,
			Description:    t.Description,
			Amount:         t.Amount,
			Kind:           kind,
			Category:       t.Category,
			PaymentMethod:  t.PaymentMethod,
		})
	}

	return analysis, nil
}

var statementDateFormats = []string{
	"2006-01-02",
	"02/01/2006",
	"2006/01/02",
	"01/02/2006",
}

func parseStatementDate(s string) (time.Time, error) {
	for _, f := range statementDateFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("formato de data não reconhecido: %s", s)
}

func toExpenseType(s string) ports.ExpenseType {
	switch s {
	case "INSTALLMENT":
		return ports.ExpenseTypeInstallment
	case "RECURRING":
		return ports.ExpenseTypeRecurring
	case "CANCEL_RECURRING":
		return ports.ExpenseTypeCancelRecurring
	case "QUERY":
		return ports.ExpenseTypeQuery
	case "EXPORT_CSV":
		return ports.ExpenseTypeExportCSV
	case "INCOME":
		return ports.ExpenseTypeIncome
	case "INCOME_RECURRING":
		return ports.ExpenseTypeIncomeRecurring
	default:
		return ports.ExpenseTypeSingle
	}
}
