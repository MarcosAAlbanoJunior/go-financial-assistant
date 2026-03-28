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
	csvExporter    usecase.CSVExporter
	messenger      ports.Messenger
	ownerPhone     string
	logger         *slog.Logger
	allowedNumbers map[string]struct{}
	sentIDs        sync.Map
	processedIDs   sync.Map
}

func newWebhookHandler(cfg ServerConfig, analyzeExpense usecase.ExpenseAnalyzer, csvExporter usecase.CSVExporter, messenger ports.Messenger, logger *slog.Logger) *webhookHandler {
	allowed := make(map[string]struct{}, len(cfg.AllowedNumbers)+1)
	for k := range cfg.AllowedNumbers {
		allowed[k] = struct{}{}
	}
	allowed[cfg.OwnerPhone+"@s.whatsapp.net"] = struct{}{}

	return &webhookHandler{
		analyzeExpense: analyzeExpense,
		csvExporter:    csvExporter,
		messenger:      messenger,
		ownerPhone:     cfg.OwnerPhone,
		logger:         logger,
		allowedNumbers: allowed,
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
			h.sentIDs.Range(func(key, value any) bool {
				if t, ok := value.(time.Time); ok && t.Before(cutoff) {
					h.sentIDs.Delete(key)
				}
				return true
			})
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

	output, err := h.route(r.Context(), payload)
	if err != nil {
		h.handleError(w, err)
		h.notifyError(r.Context(), err)
		return
	}

	if output.Type == "EXPORT_CSV" {
		h.handleExportCommand(r.Context(), w, output.ExportMonthTime)
		return
	}

	reply := formatReply(output)

	if sentID, msgErr := h.messenger.SendText(r.Context(), h.ownerPhone, reply); msgErr != nil {
		h.logger.Error("erro ao enviar resposta", "error", msgErr)
	} else if sentID != "" {
		h.sentIDs.Store(sentID, time.Now())
	}

	h.writeJSON(w, http.StatusCreated, output)
}

func (h *webhookHandler) notifyError(ctx context.Context, err error) {
	msg := "Não consegui registrar a despesa: " + err.Error()
	if sentID, msgErr := h.messenger.SendText(ctx, h.ownerPhone, msg); msgErr != nil {
		h.logger.Error("erro ao enviar notificação de erro", "error", msgErr)
	} else if sentID != "" {
		h.sentIDs.Store(sentID, time.Now())
	}
}

func (h *webhookHandler) sendText(ctx context.Context, msg string) {
	if sentID, err := h.messenger.SendText(ctx, h.ownerPhone, msg); err != nil {
		h.logger.Error("erro ao enviar mensagem", "error", err)
	} else if sentID != "" {
		h.sentIDs.Store(sentID, time.Now())
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
