package chat

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/ledger"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/openfinance"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

const pendingImportTTL = 30 * time.Minute

var (
	ErrUnsupportedMessage = errors.New("tipo de mensagem não suportado")
	ErrInvalidImage       = errors.New("imagem inválida")
)

type pendingImportSession struct {
	items     []ledger.PendingTransaction
	index     int
	expiresAt time.Time
}

// Syncer sincroniza as transações do Open Finance sob demanda.
type Syncer interface {
	Sync(ctx context.Context) (openfinance.SyncResult, error)
}

// Handler processa mensagens do dono e responde para o mesmo chat (owner).
type Handler struct {
	analyzeExpense ledger.ExpenseAnalyzer
	csvExporter    ledger.CSVExporter
	messenger      ports.Messenger
	owner          string
	logger         *slog.Logger

	syncer   Syncer   // nil quando o Open Finance não está configurado
	digester Digester // nil sem o resumo semanal
	balancer Balancer // nil sem o painel de saldos

	mu      sync.Mutex
	pending *pendingImportSession
}

func NewHandler(analyzeExpense ledger.ExpenseAnalyzer, csvExporter ledger.CSVExporter, messenger ports.Messenger, owner string, logger *slog.Logger) *Handler {
	return &Handler{
		analyzeExpense: analyzeExpense,
		csvExporter:    csvExporter,
		messenger:      messenger,
		owner:          owner,
		logger:         logger,
	}
}

// SetSyncer habilita o comando de sincronização do Open Finance.
func (h *Handler) SetSyncer(s Syncer) { h.syncer = s }

// Handle processa a mensagem e já responde ao usuário. O output só é retornado
// para registros de texto/imagem; os demais fluxos retornam (nil, nil).
// Erros de registro são notificados ao usuário e também devolvidos ao adapter.
func (h *Handler) Handle(ctx context.Context, msg Message) (*ledger.ExpenseOutput, error) {
	if msg.Document != nil {
		h.handleDocumentImport(ctx, msg.Document)
		return nil, nil
	}

	if h.tryHandlePendingConfirmation(ctx, msg.Text) {
		return nil, nil
	}

	if isSyncCommand(msg.Text) {
		h.handleSync(ctx)
		return nil, nil
	}

	if isBalancesCommand(msg.Text) {
		h.handleBalances(ctx)
		return nil, nil
	}

	if isDigestCommand(msg.Text) {
		h.handleDigest(ctx)
		return nil, nil
	}

	output, err := h.route(ctx, msg)
	if err != nil {
		h.sendText(ctx, "Não consegui registrar a despesa: "+err.Error())
		return nil, err
	}

	if output.Type == "EXPORT_CSV" {
		return nil, h.handleExportCommand(ctx, output.ExportMonthTime)
	}

	h.sendText(ctx, formatReply(output))
	return output, nil
}

func (h *Handler) route(ctx context.Context, msg Message) (*ledger.ExpenseOutput, error) {
	if msg.Image != nil {
		data, err := msg.Image.Load(ctx)
		if err != nil {
			h.logger.Error("falha ao obter imagem", "error", err)
			return nil, ErrInvalidImage
		}
		return h.analyzeExpense.ExecuteImage(ctx, ledger.ImageInput{
			ImageData: data,
			MimeType:  msg.Image.MimeType,
			Caption:   msg.Image.Caption,
		})
	}

	if msg.Text == "" {
		return nil, ErrUnsupportedMessage
	}
	return h.analyzeExpense.ExecuteText(ctx, ledger.TextInput{Text: msg.Text})
}

func (h *Handler) handleDocumentImport(ctx context.Context, doc *Attachment) {
	data, err := doc.Load(ctx)
	if err != nil {
		h.logger.Error("falha ao obter documento", "error", err)
		h.sendText(ctx, "❌ Não consegui ler o documento. Tente enviar novamente.")
		return
	}

	mimeType := doc.MimeType
	if mimeType == "" {
		mimeType = "application/pdf"
	}

	h.sendText(ctx, "⏳ Analisando o extrato, aguarde...")

	result, err := h.analyzeExpense.ExecuteDocument(ctx, ledger.DocumentInput{
		Data:     data,
		MimeType: mimeType,
		Caption:  doc.Caption,
	})
	if err != nil {
		h.logger.Error("erro ao processar extrato", "error", err)
		h.sendText(ctx, "❌ Não consegui processar o extrato. Tente novamente.")
		return
	}

	h.sendText(ctx, formatStatementSummary(result))

	if len(result.Pending) > 0 {
		h.mu.Lock()
		h.pending = &pendingImportSession{
			items:     result.Pending,
			expiresAt: time.Now().Add(pendingImportTTL),
		}
		h.mu.Unlock()
		h.sendText(ctx, formatConfirmationQuestion(result.Pending[0], 1, len(result.Pending)))
	}
}

func (h *Handler) tryHandlePendingConfirmation(ctx context.Context, text string) bool {
	answer := strings.ToLower(strings.TrimSpace(text))
	if answer != "sim" && answer != "s" && answer != "não" && answer != "nao" && answer != "n" {
		return false
	}

	h.mu.Lock()
	session := h.pending
	if session == nil || time.Now().After(session.expiresAt) {
		h.pending = nil
		h.mu.Unlock()
		return false
	}
	current := session.items[session.index]
	session.index++
	done := session.index >= len(session.items)
	if done {
		h.pending = nil
	} else {
		session.expiresAt = time.Now().Add(pendingImportTTL)
	}
	h.mu.Unlock()

	if answer == "sim" || answer == "s" {
		if err := h.analyzeExpense.SavePendingTransaction(ctx, current); err != nil {
			h.logger.Error("erro ao salvar transação confirmada", "description", current.Description, "error", err)
			h.sendText(ctx, "⚠️ Não consegui salvar: "+current.Description)
		}
	}

	if done {
		h.sendText(ctx, "✅ Importação concluída!")
	} else {
		h.sendText(ctx, formatConfirmationQuestion(session.items[session.index], session.index+1, len(session.items)))
	}
	return true
}

func (h *Handler) sendText(ctx context.Context, msg string) {
	if _, err := h.messenger.SendText(ctx, h.owner, msg); err != nil {
		h.logger.Error("erro ao enviar mensagem", "error", err)
	}
}
