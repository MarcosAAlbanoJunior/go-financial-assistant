package app

import (
	"fmt"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/config"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/db"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/setup"
)

// loadConfig abre o banco primeiro (as configurações salvas no dashboard valem mais que o ambiente) e então carrega a
// configuração. Uma configuração salva que deixe o app inválido nunca pode impedi-lo de subir (e de ser corrigida):
// nesse caso, vale só o ambiente.
func (a *app) loadConfig() error {
	databaseURL, secretKey, err := config.Bootstrap()
	if err != nil {
		return err
	}
	if a.db, err = db.NewPostgres(a.ctx, databaseURL); err != nil {
		return fmt.Errorf("banco: %w", err)
	}

	cipher, err := settings.NewCipher(secretKey)
	if err != nil {
		return fmt.Errorf("APP_SECRET_KEY inválida: %w", err)
	}
	a.store = db.NewSettingsStore(a.db)
	overrides, err := settings.LoadOverrides(a.ctx, a.store, cipher, a.logger)
	if err != nil {
		a.logger.Warn("configurações salvas no dashboard não carregadas (a migration 016 foi aplicada?)", "error", err)
	}

	if a.cfg, err = config.Load(overrides); err != nil {
		a.logger.Error("configuração salva no dashboard inválida; usando só o ambiente", "error", err)
		if a.cfg, err = config.Load(nil); err != nil {
			return err
		}
	}

	a.settings = settings.NewService(a.store, cipher, a.logger)
	if err := a.settings.Load(a.ctx); err != nil {
		a.logger.Warn("configurações do dashboard indisponíveis", "error", err)
	}
	a.onChange = func() <-chan struct{} {
		ch := make(chan struct{}, 1)
		a.settings.OnChange(func() {
			select {
			case ch <- struct{}{}:
			default:
			}
		})
		return ch
	}
	a.applyTimezone()
	a.loadSetup(cipher)
	return nil
}

// loadSetup lê o estado do setup pelo navegador. Aberto (sem senha ou sem canal, ou SETUP_REOPEN), o dashboard só
// serve o setup até ele terminar.
func (a *app) loadSetup(cipher *settings.Cipher) {
	a.channelConfigured.Store(a.cfg.ChannelConfigured())
	a.setup = setup.NewService(db.NewSetupStore(a.db), cipher, setup.Env{
		Token: a.cfg.SetupToken, Reopen: a.cfg.SetupReopen, EnvPassword: a.cfg.DashboardPassword, ChannelReady: a.channelConfigured.Load,
	}, a.logger)
	if err := a.setup.Load(a.ctx); err != nil {
		a.logger.Warn("dados do setup indisponíveis (a migration 019 foi aplicada?)", "error", err)
	}
	if !a.setup.Open() {
		return
	}
	if err := a.setup.TokenProblem(); err != nil {
		a.logger.Warn("setup pelo navegador aberto, mas ainda não pode começar", "motivo", err.Error())
		return
	}
	a.logger.Info("setup pelo navegador aberto: abra o dashboard e cole o token do .env (linha SETUP_TOKEN)", "reaberto", a.setup.Reopen())
}
