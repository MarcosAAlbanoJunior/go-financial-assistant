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
	_ "time/tzdata" // fusos horários embutidos: a imagem não precisa de tzdata do sistema

	"github.com/mdp/qrterminal/v3"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/chat"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/config"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/db"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/evolution"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/gemini"
	httpserver "github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/http"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/logo"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/pluggy"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/telegram"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// O banco abre primeiro: as configurações salvas no dashboard valem mais que o ambiente.
	databaseURL, secretKey, err := config.Bootstrap()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	postgresDB, err := db.NewPostgres(ctx, databaseURL)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer postgresDB.Close()

	cipher, err := settings.NewCipher(secretKey)
	if err != nil {
		slog.Error("APP_SECRET_KEY inválida", "error", err)
		os.Exit(1)
	}
	settingsStore := db.NewSettingsStore(postgresDB)
	if overrides, err := settings.LoadOverrides(ctx, settingsStore, cipher, logger); err != nil {
		slog.Warn("configurações salvas no dashboard não carregadas (a migration 016 foi aplicada?)", "error", err)
	} else {
		config.SetOverrides(overrides)
	}

	cfg, err := config.Load()
	if err != nil {
		// Uma configuração salva no dashboard que deixou o app inválido nunca pode impedi-lo de subir (e de ser corrigida).
		slog.Error("configuração salva no dashboard inválida; usando só o ambiente", "error", err)
		config.SetOverrides(nil)
		if cfg, err = config.Load(); err != nil {
			slog.Error("failed to load config", "error", err)
			os.Exit(1)
		}
	}

	settingsSvc := settings.NewService(settingsStore, cipher, logger)
	if err := settingsSvc.Load(ctx); err != nil {
		slog.Warn("configurações do dashboard indisponíveis", "error", err)
	}
	// onChange devolve um canal que acorda quem lê a configuração quando ela muda.
	onChange := func() <-chan struct{} {
		ch := make(chan struct{}, 1)
		settingsSvc.OnChange(func() {
			select {
			case ch <- struct{}{}:
			default:
			}
		})
		return ch
	}

	// A imagem roda em UTC: sem isto, horários e "hoje" nas mensagens e nos vencimentos saem 3 h adiantados.
	if cfg.DigestLocation != nil {
		time.Local = cfg.DigestLocation
	}

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

	// O Open Finance se liga e desliga com o app rodando: sem credenciais a sincronização apenas espera (ErrNotConfigured).
	pluggyClient := pluggy.NewClient(cfg.PluggyClientID, cfg.PluggyClientSecret)
	syncUC := usecase.NewSyncOpenFinance(purchaseRepo, pluggyClient, nil, cfg.OpenFinanceLookbackDays, logger)
	syncUC.SetLogoFetcher(logo.New())
	var syncer chat.Syncer = syncUC
	go runOpenFinanceSync(ctx, syncUC, pluggyClient, settingsSvc, onChange())

	dashboardReader := db.NewDashboardReader(postgresDB)
	insights := usecase.NewInsights(dashboardReader)
	server := httpserver.NewServer(cfg.Port, logger)
	geminiClient.SetCoachModel(settingsSvc.Get("COACH_GEMINI_MODEL"))
	settingsSvc.OnChange(func() { geminiClient.SetCoachModel(settingsSvc.Get("COACH_GEMINI_MODEL")) })
	server.SetCoach(geminiClient, cfg.GeminiPaidPlan)
	server.SetSyncer(syncer)
	settingsDeps := &httpserver.SettingsDeps{
		Service: settingsSvc,
		Channel: cfg.Channel,
		Audit:   settingsStore,
		Cleaner: db.NewTransferCleaner(postgresDB),
		PluggyCheck: func(ctx context.Context, id, secret string, items []string) ([]pluggy.ItemCheck, error) {
			return pluggy.NewClient(id, secret).Check(ctx, items)
		},
		TelegramPing: func(ctx context.Context, token string) (string, error) { return telegram.NewClient(token).GetMe(ctx) },
		GeminiPing:   gemini.Ping,
		Restart:      cancel, // o Docker (restart: unless-stopped) sobe o app de novo com as configurações novas
	}
	server.SetSettings(settingsDeps)
	if cfg.DashboardPassword != "" {
		if err := server.MountAPI(cfg.DashboardPassword, dashboardReader); err != nil {
			slog.Error("failed to mount dashboard API", "error", err)
			os.Exit(1)
		}
		slog.Info("API do dashboard ativa", "coach", cfg.GeminiPaidPlan)
	}

	var (
		messenger ports.Messenger
		owner     string
	)
	switch cfg.Channel {
	case config.ChannelTelegram:
		tg := telegram.NewClient(cfg.TelegramBotToken)
		username, err := tg.GetMe(ctx)
		if err != nil {
			// O dashboard continua no ar para o token poder ser corrigido na página de configurações.
			slog.Error("falha ao validar TELEGRAM_BOT_TOKEN: o canal fica desligado, corrija o token e reinicie", "error", err)
			break
		}
		slog.Info("canal Telegram ativo", "bot", username)

		messenger, owner = tg, strconv.FormatInt(cfg.TelegramChatID, 10)
		handler := chat.NewHandler(analyzeExpense, exportCSV, tg, owner, logger)
		handler.SetSyncer(syncer)
		handler.SetDigester(insights)
		handler.SetBalancer(insights)
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
		}, evolutionClient, analyzeExpense, exportCSV, syncer)
	}

	var monthlyReport *usecase.MonthlyReport
	if messenger != nil {
		// Avisos de segurança das configurações (senha errada, segredo trocado) vão para o mesmo chat.
		settingsDeps.Notify = func(ctx context.Context, text string) {
			if _, err := messenger.SendText(ctx, owner, text); err != nil {
				logger.Error("erro ao enviar aviso de segurança", "error", err)
			}
		}
		go usecase.NewDigestJob(insights, messenger, owner, logger, settingsSvc.Digest, onChange()).Run(ctx)
		monthlyReport = usecase.NewMonthlyReport(exportCSV, messenger, owner, logger)
	}

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
				if monthlyReport != nil && time.Now().UTC().Day() == 1 {
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

// runOpenFinanceSync sincroniza quando o intervalo vence, relendo a configuração a cada minuto e quando ela muda: credenciais,
// bancos, janela de datas e nomes próprios valem sem reiniciar. Sem credenciais ou bancos, apenas espera.
func runOpenFinanceSync(ctx context.Context, sync *usecase.SyncOpenFinance, client *pluggy.Client, svc *settings.Service, changed <-chan struct{}) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	var (
		last      time.Time
		lastCreds string
		announced bool
	)
	for {
		s := svc.Sync()
		client.SetOwnNames(svc.List("OWN_NAMES"))
		if creds := s.ClientID + "\x00" + s.ClientSecret; creds != lastCreds {
			client.SetCredentials(s.ClientID, s.ClientSecret)
			lastCreds = creds
		}
		sync.SetConfig(s.ItemIDs, s.LookbackDays)

		if s.Configured() {
			if !announced {
				slog.Info("Open Finance ativo", "items", len(s.ItemIDs), "interval", s.Interval.String())
				announced = true
			}
			if last.IsZero() || time.Since(last) >= s.Interval {
				result, err := sync.Sync(ctx)
				last = time.Now()
				if err != nil {
					slog.Error("erro ao sincronizar Open Finance", "error", err)
				}
				slog.Info("sincronização do Open Finance concluída",
					"inserted", result.Inserted, "reconciled", result.Reconciled, "existing", result.Existing)
			}
		} else {
			announced = false
		}

		select {
		case <-ctx.Done():
			return
		case <-changed:
		case <-ticker.C:
		}
	}
}
