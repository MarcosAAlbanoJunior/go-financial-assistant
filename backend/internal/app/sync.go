package app

import (
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/logo"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/pluggy"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

// startSync liga o Open Finance, que se liga e desliga com o app rodando: sem credenciais a sincronização apenas espera.
func (a *app) startSync() error {
	client := pluggy.NewClient(a.cfg.PluggyClientID, a.cfg.PluggyClientSecret)
	a.syncer = usecase.NewSyncOpenFinance(a.ledger, client, nil, a.cfg.OpenFinanceLookbackDays, a.logger)
	a.syncer.SetLogoFetcher(logo.New())
	go a.runSync(client, a.onChange())
	return nil
}

// runSync sincroniza quando o intervalo vence, relendo a configuração a cada minuto e quando ela muda: credenciais, bancos,
// janela de datas e nomes próprios valem sem reiniciar.
func (a *app) runSync(client *pluggy.Client, changed <-chan struct{}) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	var (
		last      time.Time
		lastCreds string
		announced bool
	)
	for {
		s := a.settings.Sync()
		client.SetOwnNames(a.settings.List("OWN_NAMES"))
		if creds := s.ClientID + "\x00" + s.ClientSecret; creds != lastCreds {
			client.SetCredentials(s.ClientID, s.ClientSecret)
			lastCreds = creds
		}
		a.syncer.SetConfig(s.ItemIDs, s.LookbackDays)

		if s.Configured() {
			if !announced {
				a.logger.Info("Open Finance ativo", "items", len(s.ItemIDs), "interval", s.Interval.String())
				announced = true
			}
			if last.IsZero() || time.Since(last) >= s.Interval {
				result, err := a.syncer.Sync(a.ctx)
				last = time.Now()
				if err != nil {
					a.logger.Error("erro ao sincronizar Open Finance", "error", err)
				}
				a.logger.Info("sincronização do Open Finance concluída",
					"inserted", result.Inserted, "reconciled", result.Reconciled, "existing", result.Existing)
			}
		} else {
			announced = false
		}

		select {
		case <-a.ctx.Done():
			return
		case <-changed:
		case <-ticker.C:
		}
	}
}
