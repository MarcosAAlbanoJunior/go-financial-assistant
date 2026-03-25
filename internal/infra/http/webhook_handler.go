package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

type webhookHandler struct {
	analyzeExpense usecase.ExpenseAnalyzer
	messenger      ports.Messenger
	ownerPhone     string
	logger         *slog.Logger
	allowedNumbers map[string]struct{}
	sentIDs        sync.Map
	processedIDs   sync.Map
}

func newWebhookHandler(cfg ServerConfig, analyzeExpense usecase.ExpenseAnalyzer, messenger ports.Messenger, logger *slog.Logger) *webhookHandler {
	return &webhookHandler{
		analyzeExpense: analyzeExpense,
		messenger:      messenger,
		ownerPhone:     cfg.OwnerPhone,
		logger:         logger,
		allowedNumbers: cfg.AllowedNumbers,
	}
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
			h.processedIDs.Range(func(key, value any) bool {
				if t, ok := value.(time.Time); ok && t.Before(cutoff) {
					h.processedIDs.Delete(key)
				}
				return true
			})
		}
	}
}

type evolutionPayload struct {
	Instance string        `json:"instance"`
	Data     evolutionData `json:"data"`
}

type evolutionData struct {
	Message evolutionMessage `json:"message"`
	Key     evolutionKey     `json:"key"`
	Base64  string           `json:"base64"`
}

type evolutionKey struct {
	RemoteJID string `json:"remoteJid"`
	FromMe    bool   `json:"fromMe"`
	ID        string `json:"id"`
}

type evolutionMessage struct {
	Conversation        string                 `json:"conversation,omitempty"`
	ImageMessage        *evolutionImageMessage `json:"imageMessage,omitempty"`
	ExtendedTextMessage *evolutionExtendedText `json:"extendedTextMessage,omitempty"`
}

type evolutionImageMessage struct {
	Mimetype string `json:"mimetype"`
	Caption  string `json:"caption"`
	URL      string `json:"url"`
}

type evolutionExtendedText struct {
	Text string `json:"text"`
}

func (h *webhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Webhook call")

	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		h.writeError(w, "erro ao ler body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload evolutionPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.writeError(w, "payload inválido", http.StatusBadRequest)
		return
	}

	msgID := payload.Data.Key.ID
	from := payload.Data.Key.RemoteJID

	h.logger.Info("webhook recebido",
		"instance", payload.Instance,
		"from", from,
		"id", msgID,
	)

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
		h.logger.Info("mensagem ignorada", "from", from)
		w.WriteHeader(http.StatusOK)
		return
	}

	output, err := h.route(r.Context(), payload)
	if err != nil {
		h.handleError(w, err)
		h.notifyError(r.Context(), err)
		return
	}

	reply := formatReply(output)

	if sentID, msgErr := h.messenger.SendText(r.Context(), h.ownerPhone, reply); msgErr != nil {
		h.logger.Error("erro ao enviar resposta", "error", msgErr)
	} else if sentID != "" {
		h.sentIDs.Store(sentID, struct{}{})
	}

	h.writeJSON(w, http.StatusCreated, output)
}

func (h *webhookHandler) notifyError(ctx context.Context, err error) {
	msg := "Não consegui registrar a despesa: " + err.Error()
	if sentID, msgErr := h.messenger.SendText(ctx, h.ownerPhone, msg); msgErr != nil {
		h.logger.Error("erro ao enviar notificação de erro", "error", msgErr)
	} else if sentID != "" {
		h.sentIDs.Store(sentID, struct{}{})
	}
}

func (h *webhookHandler) route(ctx context.Context, payload evolutionPayload) (*usecase.ExpenseOutput, error) {
	msg := payload.Data.Message

	if msg.ImageMessage != nil {
		return h.handleImage(ctx, payload)
	}

	text := msg.Conversation
	if msg.ExtendedTextMessage != nil {
		text = msg.ExtendedTextMessage.Text
	}

	if text == "" {
		return nil, errUnsupportedMessage
	}

	return h.handleText(ctx, text)
}

func (h *webhookHandler) handleText(ctx context.Context, text string) (*usecase.ExpenseOutput, error) {
	return h.analyzeExpense.ExecuteText(ctx, usecase.TextInput{Text: text})
}

func (h *webhookHandler) handleImage(ctx context.Context, payload evolutionPayload) (*usecase.ExpenseOutput, error) {
	img := payload.Data.Message.ImageMessage

	var imageData []byte
	var err error

	base64Data := payload.Data.Base64
	if base64Data == "" {
		h.logger.Info("base64 ausente no webhook, buscando via API")
		key := payload.Data.Key
		base64Data, err = h.messenger.FetchImageBase64(ctx, key.RemoteJID, key.FromMe, key.ID)
		if err != nil {
			h.logger.Error("falha ao buscar base64 via API", "error", err)
			return nil, errInvalidImage
		}
	}

	imageData, err = decodeBase64Image(base64Data)
	if err != nil {
		h.logger.Error("falha ao decodificar base64", "error", err)
		return nil, errInvalidImage
	}

	return h.analyzeExpense.ExecuteImage(ctx, usecase.ImageInput{
		ImageData: imageData,
		MimeType:  img.Mimetype,
		Caption:   img.Caption,
	})
}
