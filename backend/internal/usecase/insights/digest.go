package insights

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/format"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/review"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/planning"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

const (
	maxDigestNew    = 3  // contas novas citadas (as de maior valor)
	maxDigestAlerts = 12 // teto de linhas de atenção, para a mensagem caber no Telegram
)

// WeeklyDigest monta o resumo semanal: o que merece atenção agora, calculado só por código (sem IA).
func (i *Insights) WeeklyDigest(ctx context.Context, today time.Time) (string, error) {
	month := MonthStart(today)
	totals, err := i.reader.MonthlyTotals(ctx, month, month)
	if err != nil || len(totals) != 1 {
		return "", fmt.Errorf("totais do mês indisponíveis: %w", err)
	}
	rv, err := i.Review(ctx, month)
	if err != nil {
		return "", err
	}
	goals, _, err := i.Goals(ctx, today)
	if err != nil {
		return "", err
	}
	savings, err := i.Savings(ctx, month)
	if err != nil {
		return "", err
	}
	return FormatDigest(today, totals[0], rv, goals, savings), nil
}

// FormatDigest escreve o resumo em texto simples (sem Markdown, já que nomes de contas vêm do banco).
func FormatDigest(today time.Time, totals domain.MonthTotals, rv review.Review, goals []planning.GoalProgress, savings []review.DecisionResult) string {
	var alerts []string
	add := func(format string, args ...any) { alerts = append(alerts, "• "+fmt.Sprintf(format, args...)) }

	for _, d := range savings {
		if d.Status == review.SavingReturned {
			add("A cobrança voltou: %s (você tinha marcado como cancelada; foram %s neste mês).", cleanDigestLabel(d.Decision.Label), format.FormatBRL(d.Returned))
		}
	}
	newCount := 0
	for _, c := range rv.Candidates {
		if c.Dismissed {
			continue
		}
		switch {
		case c.Kind == review.ReviewDuplicate:
			add("Possível cobrança duplicada: %s (%d vezes de %s em poucos dias).", cleanDigestLabel(c.Label), c.Count, format.FormatBRL(c.Amount))
		case c.Kind == review.ReviewNew && newCount < maxDigestNew:
			newCount++
			add("Conta nova neste mês: %s (%s).", cleanDigestLabel(c.Label), format.FormatBRL(c.Amount))
		}
	}
	for _, g := range goals {
		switch g.Goal.Kind {
		case domain.GoalCut:
			if g.Current > g.Target {
				add("Meta \"%s\": o mês já passou %s do teto de %s.", g.Goal.Name, format.FormatBRL(g.Current-g.Target), format.FormatBRL(g.Target))
			} else if g.Projected != nil && *g.Projected > g.Target {
				add("Meta \"%s\": no ritmo atual o mês fecha em %s, acima do teto de %s.", g.Goal.Name, format.FormatBRL(*g.Projected), format.FormatBRL(g.Target))
			}
		case domain.GoalSave:
			if !g.Done && g.Fits != nil && !*g.Fits {
				add("Meta \"%s\": guardar %s por mês não cabe na sobra projetada de %s.", g.Goal.Name, format.FormatBRL(g.PerMonth), format.FormatBRL(max(0, *g.Surplus)))
			}
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Resumo semanal — %s\n\n", today.Format("02/01/2006"))
	fmt.Fprintf(&b, "Mês até agora: despesas %s · receitas %s\n", format.FormatBRL(totals.Expense), format.FormatBRL(totals.Income))
	if realized, perMonth := review.SavingsTotals(savings); realized > 0 {
		fmt.Fprintf(&b, "Economia realizada com o que você cancelou: %s (%s por mês).\n", format.FormatBRL(realized), format.FormatBRL(perMonth))
	}
	b.WriteString("\n")
	if len(alerts) == 0 {
		b.WriteString("Sem alertas esta semana.")
		return b.String()
	}
	b.WriteString("Atenção\n")
	if len(alerts) > maxDigestAlerts {
		alerts = alerts[:maxDigestAlerts]
	}
	b.WriteString(strings.Join(alerts, "\n"))
	return b.String()
}

// cleanDigestLabel tira a marcação de parcela do fim e limita o tamanho do nome.
func cleanDigestLabel(label string) string {
	label = planning.StripInstallment(label)
	if r := []rune(label); len(r) > 60 {
		label = string(r[:60])
	}
	return label
}
