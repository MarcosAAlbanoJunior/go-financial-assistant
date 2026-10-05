package app

import (
	"context"
	"fmt"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/db"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/gemini"
	httpserver "github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/http"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/pluggy"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/telegram"
)

// mountDashboard cria o servidor HTTP e registra a API do dashboard, a página de configurações e o setup pelo navegador.
// Enquanto o setup estiver aberto, a API só atende o setup.
func (a *app) mountDashboard() error {
	a.server = httpserver.NewServer(a.cfg.Port, a.logger)
	a.server.SetCoach(a.gemini, a.cfg.GeminiPaidPlan)
	a.server.SetSyncer(a.syncer)
	a.settingsDeps = &httpserver.SettingsDeps{
		Service: a.settings, // o canal da página vem da configuração CHANNEL, que o setup pode mudar
		Audit:   a.store,
		Cleaner: db.NewTransferCleaner(a.db),
		PluggyCheck: func(ctx context.Context, id, secret string, items []string) ([]pluggy.ItemCheck, error) {
			return pluggy.NewClient(id, secret).Check(ctx, items)
		},
		TelegramPing: func(ctx context.Context, token string) (string, error) { return telegram.NewClient(token).GetMe(ctx) },
		GeminiPing:   gemini.Ping,
		Restart:      a.cancel, // o Docker (restart: unless-stopped) sobe o app de novo com as configurações novas
	}
	a.server.SetSettings(a.settingsDeps)
	// O envio do código é ligado em startJobs, quando o canal sobe.
	a.secondFactor = &httpserver.SecondFactor{Enabled: a.cfg.DashboardTwoFactor}
	a.server.SetSecondFactor(a.secondFactor)
	a.server.SetSetup(&httpserver.SetupDeps{Service: a.setup, Telegram: setupTelegram{a}, Activate: a.activateChannel, Audit: a.store})

	// A senha vem do setup (hash no banco) ou do ambiente; sem nenhuma das duas o setup está aberto e o login não atende.
	if err := a.server.MountAPI(a.setup.CheckPassword, db.NewDashboardReader(a.db)); err != nil {
		return fmt.Errorf("API do dashboard: %w", err)
	}
	a.logger.Info("API do dashboard ativa", "coach", a.cfg.GeminiPaidPlan, "setup_aberto", a.setup.Open())
	return nil
}
