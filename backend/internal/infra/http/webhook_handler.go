package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/chat"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

// EvolutionClient é o que o adapter WhatsApp precisa da Evolution API.
type EvolutionClient interface {
	ports.Messenger
	QRProvider
	FetchImageBase64(ctx context.Context, remoteJid string, fromMe bool, messageID string) (string, error)
}

// webhookHandler é o adapter do WhatsApp: traduz o payload da Evolution API para
// chat.Message e delega a conversa ao chat.Handler.
type webhookHandler struct {
	chat           *chat.Handler
	media          EvolutionClient
	logger         *slog.Logger
	allowedNumbers map[string]struct{}
	sentIDs        sync.Map
	processedIDs   sync.Map
}

func newWebhookHandler(cfg WhatsAppConfig, client EvolutionClient, analyzeExpense usecase.ExpenseAnalyzer, csvExporter usecase.CSVExporter, logger *slog.Logger) *webhookHandler {
	allowed := make(map[string]struct{}, len(cfg.AllowedNumbers)+1)
	for k := range cfg.AllowedNumbers {
		allowed[k] = struct{}{}
	}
	allowed[cfg.OwnerPhone+"@s.whatsapp.net"] = struct{}{}

	h := &webhookHandler{media: client, logger: logger, allowedNumbers: allowed}
	h.chat = chat.NewHandler(analyzeExpense, csvExporter, echoTracker{client, &h.sentIDs}, cfg.OwnerPhone, logger)
	return h
}

// echoTracker registra os IDs das mensagens enviadas pelo bot: como o usuário
// conversa consigo mesmo, a Evolution devolve cada resposta como novo webhook.
type echoTracker struct {
	ports.Messenger
	sentIDs *sync.Map
}

func (t echoTracker) SendText(ctx context.Context, to, text string) (string, error) {
	return t.track(t.Messenger.SendText(ctx, to, text))
}

func (t echoTracker) SendDocument(ctx context.Context, to, filename string, data []byte, caption string) (string, error) {
	return t.track(t.Messenger.SendDocument(ctx, to, filename, data, caption))
}

func (t echoTracker) track(id string, err error) (string, error) {
	if err == nil && id != "" {
		t.sentIDs.Store(id, time.Now())
	}
	return id, err
}

func (h *webhookHandler) startCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-time.Hour)
			for _, m := range []*sync.Map{&h.processedIDs, &h.sentIDs} {
				m.Range(func(key, value any) bool {
					if t, ok := value.(time.Time); ok && t.Before(cutoff) {
						m.Delete(key)
					}
					return true
				})
			}
		}
	}
}

func (h *webhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		h.writeError(w, "erro ao ler body", http.StatusBadRequest)
		return
	}

	var envelope evolutionEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		h.writeError(w, "payload inválido", http.StatusBadRequest)
		return
	}

	if envelope.Event != "" && envelope.Event != "messages.upsert" {
		h.logger.Info("evento ignorado", "event", envelope.Event)
		w.WriteHeader(http.StatusOK)
		return
	}

	var payload evolutionPayload
	payload.Instance = envelope.Instance
	if err := json.Unmarshal(envelope.Data, &payload.Data); err != nil {
		h.writeError(w, "payload inválido", http.StatusBadRequest)
		return
	}

	msgID := payload.Data.Key.ID
	from := payload.Data.Key.RemoteJID

	h.logger.Info("webhook recebido", "instance", payload.Instance, "from", maskPhone(from), "id", msgID)

	if _, isSentByBot := h.sentIDs.LoadAndDelete(msgID); isSentByBot {
		w.WriteHeader(http.StatusOK)
		return
	}

	if msgID != "" {
		if _, already := h.processedIDs.LoadOrStore(msgID, time.Now()); already {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	if _, isAllowed := h.allowedNumbers[from]; !isAllowed {
		h.logger.Info("mensagem ignorada", "from", maskPhone(from))
		w.WriteHeader(http.StatusOK)
		return
	}

	output, err := h.chat.Handle(r.Context(), h.toChatMessage(payload))
	if err != nil {
		h.handleError(w, err)
		return
	}
	if output == nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	h.writeJSON(w, http.StatusCreated, output)
}

func (h *webhookHandler) toChatMessage(payload evolutionPayload) chat.Message {
	msg := payload.Data.Message

	load := func(ctx context.Context) ([]byte, error) {
		b64 := payload.Data.Base64
		if b64 == "" {
			h.logger.Info("base64 ausente no webhook, buscando via API")
			key := payload.Data.Key
			var err error
			if b64, err = h.media.FetchImageBase64(ctx, key.RemoteJID, key.FromMe, key.ID); err != nil {
				return nil, err
			}
		}
		return decodeBase64Image(b64)
	}

	switch {
	case msg.DocumentMessage != nil:
		return chat.Message{Document: &chat.Attachment{MimeType: msg.DocumentMessage.Mimetype, Caption: msg.DocumentMessage.Caption, Load: load}}
	case msg.ImageMessage != nil:
		return chat.Message{Image: &chat.Attachment{MimeType: msg.ImageMessage.Mimetype, Caption: msg.ImageMessage.Caption, Load: load}}
	case msg.ExtendedTextMessage != nil:
		return chat.Message{Text: msg.ExtendedTextMessage.Text}
	}
	return chat.Message{Text: msg.Conversation}
}

func (h *webhookHandler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (h *webhookHandler) writeError(w http.ResponseWriter, msg string, status int) {
	h.writeJSON(w, status, map[string]string{"error": msg})
}

func (h *webhookHandler) handleError(w http.ResponseWriter, err error) {
	h.logger.Error("erro ao processar webhook", "error", err)

	switch {
	case errors.Is(err, domain.ErrInvalidAmount):
		h.writeError(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrInvalidPaymentMethod):
		h.writeError(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, chat.ErrUnsupportedMessage):
		h.writeError(w, "tipo de mensagem não suportado", http.StatusBadRequest)
	case errors.Is(err, chat.ErrInvalidImage):
		h.writeError(w, "imagem inválida ou corrompida", http.StatusBadRequest)
	default:
		h.writeError(w, "erro interno", http.StatusInternalServerError)
	}
}
