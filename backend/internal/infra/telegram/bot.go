package telegram

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/ledger"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/chat"
)

const (
	pollTimeoutSec = 30
	retryDelay     = 5 * time.Second
	updateTimeout  = 2 * time.Minute
)

type messageHandler interface {
	Handle(ctx context.Context, msg chat.Message) (*ledger.ExpenseOutput, error)
}

// Bot recebe mensagens por long polling (sem precisar de URL pública) e as entrega ao
// chat.Handler. Só o dono (ownerID), em conversa privada, é atendido.
type Bot struct {
	client  *Client
	ownerID int64
	handler messageHandler
	logger  *slog.Logger
}

func NewBot(client *Client, ownerID int64, handler messageHandler, logger *slog.Logger) *Bot {
	return &Bot{client: client, ownerID: ownerID, handler: handler, logger: logger}
}

func (b *Bot) Run(ctx context.Context) {
	var offset int64
	for ctx.Err() == nil {
		updates, err := b.client.getUpdates(ctx, offset, pollTimeoutSec)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			b.logger.Error("erro ao buscar updates do Telegram", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryDelay):
			}
			continue
		}
		for _, u := range updates {
			// Avança o offset mesmo se o processamento falhar: um update ruim não pode travar o bot.
			offset = u.UpdateID + 1
			b.process(ctx, u)
		}
	}
}

func (b *Bot) process(ctx context.Context, u update) {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Error("panic ao processar update do Telegram", "update_id", u.UpdateID, "panic", r)
		}
	}()

	m := u.Message
	if m == nil || m.Chat.Type != "private" || m.From == nil || m.From.ID != b.ownerID {
		b.logger.Info("mensagem do Telegram ignorada", "update_id", u.UpdateID)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()

	if _, err := b.handler.Handle(ctx, b.toChatMessage(m)); err != nil {
		b.logger.Error("erro ao processar mensagem do Telegram", "update_id", u.UpdateID, "error", err)
	}
}

func (b *Bot) toChatMessage(m *message) chat.Message {
	attachment := func(fileID, mimeType string) *chat.Attachment {
		return &chat.Attachment{
			MimeType: mimeType,
			Caption:  m.Caption,
			Load:     func(ctx context.Context) ([]byte, error) { return b.client.Download(ctx, fileID) },
		}
	}

	switch {
	case len(m.Photo) > 0:
		// O último item do array é a maior resolução; fotos do Telegram são sempre JPEG.
		return chat.Message{Image: attachment(m.Photo[len(m.Photo)-1].FileID, "image/jpeg")}
	case m.Document != nil && strings.HasPrefix(m.Document.MimeType, "image/"):
		// Imagem enviada "como arquivo" é um recibo, não um extrato.
		return chat.Message{Image: attachment(m.Document.FileID, m.Document.MimeType)}
	case m.Document != nil:
		return chat.Message{Document: attachment(m.Document.FileID, m.Document.MimeType)}
	}
	return chat.Message{Text: m.Text}
}
