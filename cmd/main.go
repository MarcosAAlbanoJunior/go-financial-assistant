package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/mdp/qrterminal/v3"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/chat"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/config"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/db"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/evolution"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/gemini"
	httpserver "github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/http"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/telegram"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	postgresDB, err := db.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer postgresDB.Close()

	geminiClient, err := gemini.NewClient(ctx, cfg.GeminiAPIKey)
	if err != nil {
		slog.Error("failed to create gemini client", "error", err)
		os.Exit(1)
	}
	defer geminiClient.Close()

	purchaseRepo := db.NewPurchaseRepository(postgresDB)

	analyzeExpense := usecase.NewAnalyzeExpense(purchaseRepo, geminiClient, logger)
	exportCSV := usecase.NewExportCSV(purchaseRepo)

	if err := analyzeExpense.GenerateRecurringExpenses(ctx); err != nil {
		slog.Error("erro ao gerar despesas recorrentes no startup", "error", err)
	}

	server := httpserver.NewServer(cfg.Port, logger)

	var (
		messenger ports.Messenger
		owner     string
	)
	switch cfg.Channel {
	case config.ChannelTelegram:
		tg := telegram.NewClient(cfg.TelegramBotToken)
		username, err := tg.GetMe(ctx)
		if err != nil {
			slog.Error("falha ao validar TELEGRAM_BOT_TOKEN", "error", err)
			os.Exit(1)
		}
		slog.Info("canal Telegram ativo", "bot", username)

		messenger, owner = tg, strconv.FormatInt(cfg.TelegramChatID, 10)
		handler := chat.NewHandler(analyzeExpense, exportCSV, tg, owner, logger)
		go telegram.NewBot(tg, cfg.TelegramChatID, handler, logger).Run(ctx)
	default:
		evolutionClient := evolution.NewClient(cfg.EvolutionAPIURL, cfg.EvolutionInstance, cfg.EvolutionAPIKey)
		connectWhatsApp(ctx, evolutionClient, cfg)

		messenger, owner = evolutionClient, cfg.OwnerPhone
		server.MountWhatsApp(httpserver.WhatsAppConfig{
			OwnerPhone:      cfg.OwnerPhone,
			AllowedNumbers:  cfg.AllowedNumbers,
			EvolutionAPIURL: cfg.EvolutionAPIURL,
			AdminSecret:     cfg.AdminSecret,
		}, evolutionClient, analyzeExpense, exportCSV)
	}

	monthlyReport := usecase.NewMonthlyReport(exportCSV, messenger, owner, logger)

	go func() {
		for {
			now := time.Now().UTC()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next)):
				if err := analyzeExpense.GenerateRecurringExpenses(ctx); err != nil {
					slog.Error("erro ao gerar despesas recorrentes", "error", err)
				}
				if time.Now().UTC().Day() == 1 {
					if err := monthlyReport.Send(ctx); err != nil {
						slog.Error("erro ao enviar relatório mensal", "error", err)
					}
				}
			}
		}
	}()

	slog.Info("starting go-financial-assistant", "port", cfg.Port)

	if err := server.Start(ctx); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}

// connectWhatsApp aguarda a Evolution API subir e exibe o QR code se o WhatsApp ainda não estiver conectado.
func connectWhatsApp(ctx context.Context, evolutionClient *evolution.Client, cfg *config.Config) {
	for {
		_, err := evolutionClient.EnsureInstance(ctx, cfg.OwnerPhone)
		if err == nil {
			break
		}
		slog.Warn("Evolution API não disponível, aguardando...", "error", err)
		select {
		case <-ctx.Done():
			os.Exit(0)
		case <-time.After(5 * time.Second):
		}
	}

	time.Sleep(2 * time.Second)

	state, err := evolutionClient.FetchConnectionState(ctx)
	if err != nil {
		slog.Warn("não foi possível verificar estado da conexão", "error", err)
	} else if state != "open" {
		code, _, err := evolutionClient.FetchConnectCode(ctx)
		if err != nil {
			slog.Warn("não foi possível buscar QR code, acesse manualmente",
				"url", fmt.Sprintf("%s/instance/connect/%s", cfg.EvolutionAPIURL, cfg.EvolutionInstance))
		} else {
			qrterminal.GenerateWithConfig(code, qrterminal.Config{
				Level:      qrterminal.L,
				Writer:     os.Stdout,
				HalfBlocks: true,
			})
			fmt.Println("Escaneie o QR code acima com o WhatsApp para conectar.")
		}
	}
}
