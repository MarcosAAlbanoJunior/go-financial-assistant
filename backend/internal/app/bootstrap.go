package app

import (
	"fmt"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/config"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/db"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
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
	if overrides, err := settings.LoadOverrides(a.ctx, a.store, cipher, a.logger); err != nil {
		a.logger.Warn("configurações salvas no dashboard não carregadas (a migration 016 foi aplicada?)", "error", err)
	} else {
		config.SetOverrides(overrides)
	}

	if a.cfg, err = config.Load(); err != nil {
		a.logger.Error("configuração salva no dashboard inválida; usando só o ambiente", "error", err)
		config.SetOverrides(nil)
		if a.cfg, err = config.Load(); err != nil {
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
	return nil
}
