package app

import (
	"context"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/ledger"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

// startJobs liga o que roda em segundo plano: aviso de segurança das configurações, resumo semanal e a rotina diária.
// Sem canal de conversa (messenger nil) só a rotina diária de despesas recorrentes roda.
func (a *app) startJobs(messenger ports.Messenger, owner string) {
	var monthlyReport *ledger.MonthlyReport
	if messenger != nil {
		// Avisos de segurança das configurações (senha errada, segredo trocado) vão para o mesmo chat.
		a.settingsDeps.Notify = func(ctx context.Context, text string) {
			if _, err := messenger.SendText(ctx, owner, text); err != nil {
				a.logger.Error("erro ao enviar aviso de segurança", "error", err)
			}
		}
		go usecase.NewDigestJob(a.insights, messenger, owner, a.logger, a.settings.Digest, a.onChange()).Run(a.ctx)
		monthlyReport = ledger.NewMonthlyReport(a.exportCSV, messenger, owner, a.logger)
	}
	go a.runDaily(monthlyReport)
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
			if monthlyReport != nil && time.Now().UTC().Day() == 1 {
				if err := monthlyReport.Send(a.ctx); err != nil {
					a.logger.Error("erro ao enviar relatório mensal", "error", err)
				}
			}
		}
	}
}
