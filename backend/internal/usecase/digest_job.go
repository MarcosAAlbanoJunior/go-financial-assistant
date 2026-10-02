package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
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
	schedule  DigestSchedule
	clock     domain.Clock
	changed   <-chan struct{}
}

// DigestSchedule devolve a agenda atual; é lida a cada ciclo, então mudar a configuração vale sem reiniciar.
type DigestSchedule func() (enabled bool, weekday time.Weekday, hour int, loc *time.Location)

// NewDigestJob: changed (opcional) acorda o job quando a configuração muda, para recalcular o próximo envio.
func NewDigestJob(insights *Insights, messenger ports.Messenger, owner string, logger *slog.Logger, schedule DigestSchedule, changed <-chan struct{}) *DigestJob {
	return &DigestJob{insights: insights, messenger: messenger, owner: owner, logger: logger, schedule: schedule, changed: changed, clock: domain.SystemClock{}}
}

// Send monta e envia o resumo de hoje.
func (j *DigestJob) Send(ctx context.Context) error {
	_, _, _, loc := j.schedule()
	text, err := j.insights.WeeklyDigest(ctx, j.clock.Now().In(loc))
	if err != nil {
		return err
	}
	_, err = j.messenger.SendText(ctx, j.owner, text)
	return err
}

// Run espera cada hora marcada e envia, até o contexto acabar. Erros vão ao log sem o conteúdo do resumo.
func (j *DigestJob) Run(ctx context.Context) {
	for {
		enabled, weekday, hour, loc := j.schedule()
		if !enabled {
			select {
			case <-ctx.Done():
				return
			case <-j.changed:
			}
			continue
		}
		next := NextDigestTime(j.clock.Now(), weekday, hour, loc)
		j.logger.Info("próximo resumo semanal", "at", next.Format(time.RFC3339))
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-j.changed:
			timer.Stop()
			continue // agenda mudou: recalcula
		case <-timer.C:
		}
		if err := j.Send(ctx); err != nil {
			j.logger.Error("erro ao enviar o resumo semanal", "error", err)
		} else {
			j.logger.Info("resumo semanal enviado")
		}
	}
}
