package usecase

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

type MonthlyReport struct {
	exporter  CSVExporter
	messenger ports.Messenger
	phone     string
	logger    *slog.Logger
}

func NewMonthlyReport(exporter CSVExporter, messenger ports.Messenger, phone string, logger *slog.Logger) *MonthlyReport {
	return &MonthlyReport{
		exporter:  exporter,
		messenger: messenger,
		phone:     phone,
		logger:    logger,
	}
}

func (r *MonthlyReport) Send(ctx context.Context) error {
	prevMonth := previousMonth()

	data, filename, summary, err := r.exporter.Execute(ctx, prevMonth)
	if err != nil {
		return fmt.Errorf("erro ao gerar relatório mensal: %w", err)
	}

	if data == nil {
		r.logger.Info("sem lançamentos no mês anterior, relatório não enviado",
			"month", prevMonth.Format("01/2006"))
		return nil
	}

	caption := BuildExportCaption(prevMonth, summary)
	base64Data := base64.StdEncoding.EncodeToString(data)

	if _, err := r.messenger.SendDocument(ctx, r.phone, filename, base64Data, caption); err != nil {
		return fmt.Errorf("erro ao enviar relatório mensal: %w", err)
	}

	r.logger.Info("relatório mensal enviado", "month", prevMonth.Format("01/2006"))
	return nil
}

func previousMonth() time.Time {
	now := time.Now().UTC()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return first.AddDate(0, -1, 0)
}
