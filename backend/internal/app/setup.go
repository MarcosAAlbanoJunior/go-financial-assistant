package app

import (
	"context"
	"errors"
	"strconv"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/config"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/telegram"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/setup"
)

// activateChannel liga, sem reiniciar, o canal que o setup acabou de gravar (ou o canal mantido). Chamado depois da
// transação: se algo falhar daqui em diante, o setup segue concluído e o canal tenta subir de novo sozinho.
func (a *app) activateChannel(_ context.Context, act setup.Activation) error {
	if act.Keep {
		if a.channelStarted.Load() {
			return nil // o canal do .env já está no ar
		}
		// Reaberto mantendo o canal de antes: ele não subiu no boot (o setup estava aberto) e sobe agora.
		if a.cfg.Channel == config.ChannelTelegram {
			return a.launchChannel()
		}
		go func() {
			if err := a.launchChannel(); err != nil && !errors.Is(err, context.Canceled) {
				a.logger.Error("erro ao ligar o WhatsApp depois do setup", "error", err)
			}
		}()
		return nil
	}

	// A página de Configurações passa a mostrar o Telegram, sem pedir reinício: ele já está valendo.
	a.settings.Adopt(map[string]string{
		"CHANNEL": config.ChannelTelegram, "TELEGRAM_BOT_TOKEN": act.Token, "TELEGRAM_CHAT_ID": strconv.FormatInt(act.ChatID, 10),
	})
	a.channelConfigured.Store(true)
	if !a.channelStarted.CompareAndSwap(false, true) {
		return errChannelStarted
	}
	a.startTelegram(act.Token, act.ChatID, act.Offset)
	return nil
}

// setupTelegram é o Telegram do setup: um cliente por chamada, com o token do rascunho. Os erros viram os do setup, sem
// detalhes internos na resposta (o erro real vai ao log).
type setupTelegram struct{ a *app }

func (t setupTelegram) Bot(ctx context.Context, token string) (string, error) {
	name, err := telegram.NewClient(token).GetMe(ctx)
	return name, t.err("getMe", err)
}

func (t setupTelegram) LatestOffset(ctx context.Context, token string) (int64, error) {
	offset, err := telegram.NewClient(token).LatestOffset(ctx)
	return offset, t.err("getUpdates", err)
}

func (t setupTelegram) FirstPrivate(ctx context.Context, token string, offset int64) (*setup.Candidate, int64, error) {
	s, next, err := telegram.NewClient(token).FirstPrivate(ctx, offset)
	if err != nil || s == nil {
		return nil, next, t.err("getUpdates", err)
	}
	return &setup.Candidate{ID: s.ID, Name: s.Name, Username: s.Username}, next, nil
}

func (t setupTelegram) Send(ctx context.Context, token string, chatID int64, text string) error {
	_, err := telegram.NewClient(token).SendText(ctx, strconv.FormatInt(chatID, 10), text)
	return t.err("sendMessage", err)
}

func (t setupTelegram) err(op string, err error) error {
	if err == nil {
		return nil
	}
	var api *telegram.APIError
	if errors.As(err, &api) {
		switch api.Code {
		case 401, 404:
			return setup.ErrBotTokenInvalid
		case 409:
			return setup.ErrBotBusy
		}
	}
	t.a.logger.Warn("Telegram no setup", "op", op, "error", err)
	return setup.ErrTelegramDown
}
