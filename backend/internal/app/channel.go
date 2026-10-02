package app

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/mdp/qrterminal/v3"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/chat"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/config"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/evolution"
	httpserver "github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/http"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/telegram"
)

// startChannel liga o canal de conversa configurado e devolve por onde mandar mensagens e para quem. Se o canal não
// puder subir (token do Telegram errado), devolve messenger nil: o dashboard segue no ar para a configuração ser corrigida.
func (a *app) startChannel() (ports.Messenger, string, error) {
	if a.cfg.Channel == config.ChannelTelegram {
		return a.startTelegram()
	}
	return a.startWhatsApp()
}

func (a *app) startTelegram() (ports.Messenger, string, error) {
	tg := telegram.NewClient(a.cfg.TelegramBotToken)
	username, err := tg.GetMe(a.ctx)
	if err != nil {
		a.logger.Error("falha ao validar TELEGRAM_BOT_TOKEN: o canal fica desligado, corrija o token e reinicie", "error", err)
		return nil, "", nil
	}
	a.logger.Info("canal Telegram ativo", "bot", username)

	owner := strconv.FormatInt(a.cfg.TelegramChatID, 10)
	handler := chat.NewHandler(a.analyzeExpense, a.exportCSV, tg, owner, a.logger)
	handler.SetSyncer(a.syncer)
	handler.SetDigester(a.insights)
	handler.SetBalancer(a.insights)
	go telegram.NewBot(tg, a.cfg.TelegramChatID, handler, a.logger).Run(a.ctx)
	return tg, owner, nil
}

func (a *app) startWhatsApp() (ports.Messenger, string, error) {
	client := evolution.NewClient(a.cfg.EvolutionAPIURL, a.cfg.EvolutionInstance, a.cfg.EvolutionAPIKey)
	if err := a.connectWhatsApp(client); err != nil {
		return nil, "", err
	}
	a.server.MountWhatsApp(httpserver.WhatsAppConfig{
		OwnerPhone:      a.cfg.OwnerPhone,
		AllowedNumbers:  a.cfg.AllowedNumbers,
		EvolutionAPIURL: a.cfg.EvolutionAPIURL,
		AdminSecret:     a.cfg.AdminSecret,
	}, client, a.analyzeExpense, a.exportCSV, a.syncer)
	return client, a.cfg.OwnerPhone, nil
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
