package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/mdp/qrterminal/v3"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/chat"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/config"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/evolution"
	httpserver "github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/http"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/telegram"
)

// Espera entre as tentativas de ligar um canal que não subiu: começa curta e cresce até o teto.
var (
	channelRetryFirst = 5 * time.Second
	channelRetryMax   = 5 * time.Minute
)

const telegramConnectTimeout = 20 * time.Second

var errChannelStarted = errors.New("o canal de conversa já foi ligado")

// startChannel liga o canal configurado (no ambiente ou pelo setup). Sem canal configurado não liga nada: quem
// configura é o setup, que liga o canal ao concluir, sem reiniciar.
func (a *app) startChannel() error {
	if !a.cfg.ChannelConfigured() {
		a.logger.Warn("sem canal de conversa: conclua o setup no dashboard para ligar o Telegram")
		return nil
	}
	// Reaberto, a pessoa pode trocar o bot: o canal de antes só sobe se ela escolher mantê-lo (dois bots com o mesmo
	// token brigariam pelas mensagens).
	if a.setup.Reopen() {
		a.logger.Warn("setup reaberto (SETUP_REOPEN): o canal fica desligado até o setup terminar")
		return nil
	}
	return a.launchChannel()
}

// launchChannel liga o canal da configuração. Só uma vez por processo: dois bots fazendo polling com o mesmo token
// brigam pelas mensagens (o Telegram devolve 409 a um deles).
func (a *app) launchChannel() error {
	if !a.channelStarted.CompareAndSwap(false, true) {
		return errChannelStarted
	}
	if a.cfg.Channel == config.ChannelTelegram {
		a.startTelegram(a.cfg.TelegramBotToken, a.cfg.TelegramChatID, 0)
		return nil
	}
	return a.startWhatsApp()
}

// startTelegram confere o token e liga o bot. Se o Telegram não responder (token errado, API fora), o canal fica fora do
// ar, o login responde 503 (o segundo fator continua exigido) e o app tenta de novo em segundo plano.
func (a *app) startTelegram(token string, chatID, offset int64) {
	tg := telegram.NewClient(token)
	connect := func() error {
		ctx, cancel := context.WithTimeout(a.ctx, telegramConnectTimeout)
		defer cancel()
		username, err := tg.GetMe(ctx)
		if err == nil {
			a.logger.Info("canal Telegram ativo", "bot", username)
		}
		return err
	}
	// Tudo montado antes de publicar: quem usa o canal nunca vê um bot pela metade.
	run := func() {
		owner := strconv.FormatInt(chatID, 10)
		handler := chat.NewHandler(a.analyzeExpense, a.exportCSV, tg, owner, a.logger)
		handler.SetSyncer(a.syncer)
		handler.SetDigester(a.insights)
		handler.SetBalancer(a.insights)
		bot := telegram.NewBot(tg, chatID, handler, a.logger).StartFrom(offset)
		a.channel.publish(tg, owner)
		go bot.Run(a.ctx)
	}

	err := connect()
	if err == nil {
		run()
		return
	}
	a.logger.Error("o Telegram não respondeu: canal fora do ar (o login fica indisponível); tentando de novo em segundo plano",
		"proxima_tentativa", channelRetryFirst.String(), "error", err)
	go func() {
		failed := func(err error, next time.Duration) {
			a.logger.Error("o Telegram ainda não respondeu", "proxima_tentativa", next.String(), "error", err)
		}
		if retryBackoff(a.ctx, channelRetryFirst, channelRetryMax, connect, failed) {
			run()
		}
	}()
}

func (a *app) startWhatsApp() error {
	client := evolution.NewClient(a.cfg.EvolutionAPIURL, a.cfg.EvolutionInstance, a.cfg.EvolutionAPIKey)
	if err := a.connectWhatsApp(client); err != nil {
		return err
	}
	a.server.MountWhatsApp(httpserver.WhatsAppConfig{
		OwnerPhone:      a.cfg.OwnerPhone,
		AllowedNumbers:  a.cfg.AllowedNumbers,
		EvolutionAPIURL: a.cfg.EvolutionAPIURL,
		AdminSecret:     a.cfg.AdminSecret,
	}, client, a.analyzeExpense, a.exportCSV, a.syncer)
	a.channel.publish(client, a.cfg.OwnerPhone)
	return nil
}

// connectWhatsApp aguarda a Evolution API subir e exibe o QR code se o WhatsApp ainda não estiver conectado.
// Devolve erro só se o app for encerrado enquanto espera.
func (a *app) connectWhatsApp(client *evolution.Client) error {
	for {
		if _, err := client.EnsureInstance(a.ctx, a.cfg.OwnerPhone); err == nil {
			break
		} else {
			a.logger.Warn("Evolution API não disponível, aguardando...", "error", err)
		}
		select {
		case <-a.ctx.Done():
			return a.ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}

	time.Sleep(2 * time.Second)

	state, err := client.FetchConnectionState(a.ctx)
	if err != nil {
		a.logger.Warn("não foi possível verificar estado da conexão", "error", err)
		return nil
	}
	if state == "open" {
		return nil
	}
	code, _, err := client.FetchConnectCode(a.ctx)
	if err != nil {
		a.logger.Warn("não foi possível buscar QR code, acesse manualmente",
			"url", fmt.Sprintf("%s/instance/connect/%s", a.cfg.EvolutionAPIURL, a.cfg.EvolutionInstance))
		return nil
	}
	qrterminal.GenerateWithConfig(code, qrterminal.Config{Level: qrterminal.L, Writer: os.Stdout, HalfBlocks: true})
	fmt.Println("Escaneie o QR code acima com o WhatsApp para conectar.")
	return nil
}
