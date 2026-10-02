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

// mountDashboard cria o servidor HTTP e, havendo senha configurada, registra a API do dashboard e a página de configurações.
func (a *app) mountDashboard() error {
	a.server = httpserver.NewServer(a.cfg.Port, a.logger)
	a.server.SetCoach(a.gemini, a.cfg.GeminiPaidPlan)
	a.server.SetSyncer(a.syncer)
	a.settingsDeps = &httpserver.SettingsDeps{
		Service: a.settings,
		Channel: a.cfg.Channel,
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

	if a.cfg.DashboardPassword == "" {
		return nil
	}
	if err := a.server.MountAPI(a.cfg.DashboardPassword, db.NewDashboardReader(a.db)); err != nil {
		return fmt.Errorf("API do dashboard: %w", err)
	}
	a.logger.Info("API do dashboard ativa", "coach", a.cfg.GeminiPaidPlan)
	return nil
}
