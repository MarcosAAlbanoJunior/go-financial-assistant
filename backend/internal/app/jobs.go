package app

import (
	"context"
	"errors"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/insights"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/ledger"
)

// startJobs liga o que usa o chat (avisos de segurança, código do segundo fator, resumo semanal, relatório mensal) ao
// canal trocável, e a rotina diária. Enquanto não há canal no ar, os envios falham com ports.ErrNoChannel; quando o canal
// liga (no boot, numa nova tentativa ou ao concluir o setup), passam a funcionar sem reiniciar nada.
func (a *app) startJobs() {
	if !a.cfg.DashboardTwoFactor {
		a.logger.Warn("dashboard sem segundo fator: o login pede só a senha", "motivo", "DASHBOARD_2FA=off")
	}
	// O código do segundo fator vai ao chat. Diferente dos avisos, o envio precisa dizer se deu certo: sem o código
	// ninguém entra.
	a.secondFactor.Send = func(ctx context.Context, text string) error {
		_, err := a.channel.SendText(ctx, "", text)
		return err
	}
	a.secondFactor.Configured = a.channelConfigured.Load
	// Avisos de segurança das configurações (senha errada, segredo trocado) vão para o mesmo chat.
	a.settingsDeps.Notify = func(ctx context.Context, text string) {
		if _, err := a.channel.SendText(ctx, "", text); err != nil && !errors.Is(err, ports.ErrNoChannel) {
			a.logger.Error("erro ao enviar aviso de segurança", "error", err)
		}
	}
	go insights.NewDigestJob(a.insights, &a.channel, "", a.logger, a.settings.Digest, a.onChange()).Run(a.ctx)
	go a.runDaily(ledger.NewMonthlyReport(a.exportCSV, &a.channel, "", a.logger))
}

// runDaily gera as despesas recorrentes à meia-noite (UTC) e, no dia 1, envia o relatório mensal.
func (a *app) runDaily(monthlyReport *ledger.MonthlyReport) {
	for {
		now := time.Now().UTC()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
		select {
		case <-a.ctx.Done():
			return
		case <-time.After(time.Until(next)):
			if err := a.analyzeExpense.GenerateRecurringExpenses(a.ctx); err != nil {
				a.logger.Error("erro ao gerar despesas recorrentes", "error", err)
			}
			if time.Now().UTC().Day() == 1 {
				if err := monthlyReport.Send(a.ctx); err != nil {
					a.logger.Error("erro ao enviar relatório mensal", "error", err)
				}
			}
		}
	}
}
