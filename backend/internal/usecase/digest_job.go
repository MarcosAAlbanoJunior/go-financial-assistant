package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// NextDigestTime é o próximo instante (depois de now) em que cai o dia da semana e a hora pedidos, no fuso loc.
func NextDigestTime(now time.Time, weekday time.Weekday, hour int, loc *time.Location) time.Time {
	local := now.In(loc)
	t := time.Date(local.Year(), local.Month(), local.Day(), hour, 0, 0, 0, loc)
	for t.Weekday() != weekday || !t.After(now) {
		t = time.Date(t.Year(), t.Month(), t.Day()+1, hour, 0, 0, 0, loc)
	}
	return t
}

// DigestJob manda o resumo semanal ao dono no dia e hora combinados. Se o app estiver desligado nessa hora, o
// resumo daquela semana é pulado (nada fica guardado); "/resumo" no chat pede um na hora.
type DigestJob struct {
	insights  *Insights
	messenger ports.Messenger
	owner     string
	logger    *slog.Logger
	weekday   time.Weekday
	hour      int
	loc       *time.Location
}

func NewDigestJob(insights *Insights, messenger ports.Messenger, owner string, logger *slog.Logger, weekday time.Weekday, hour int, loc *time.Location) *DigestJob {
	return &DigestJob{insights: insights, messenger: messenger, owner: owner, logger: logger, weekday: weekday, hour: hour, loc: loc}
}

// Send monta e envia o resumo de hoje.
func (j *DigestJob) Send(ctx context.Context) error {
	text, err := j.insights.WeeklyDigest(ctx, time.Now().In(j.loc))
	if err != nil {
		return err
	}
	_, err = j.messenger.SendText(ctx, j.owner, text)
	return err
}

// Run espera cada hora marcada e envia, até o contexto acabar. Erros vão ao log sem o conteúdo do resumo.
func (j *DigestJob) Run(ctx context.Context) {
	for {
		next := NextDigestTime(time.Now(), j.weekday, j.hour, j.loc)
		j.logger.Info("próximo resumo semanal", "at", next.Format(time.RFC3339))
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}
		if err := j.Send(ctx); err != nil {
			j.logger.Error("erro ao enviar o resumo semanal", "error", err)
		} else {
			j.logger.Info("resumo semanal enviado")
		}
	}
}
