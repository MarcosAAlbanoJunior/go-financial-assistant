package usecase

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

const (
	maxDigestNew    = 3  // contas novas citadas (as de maior valor)
	maxDigestAlerts = 12 // teto de linhas de atenção, para a mensagem caber no Telegram
)

// FormatBRL escreve um valor em reais no padrão brasileiro: R$ 1.234,56.
func FormatBRL(v float64) string {
	cents := int64(math.Round(math.Abs(v) * 100))
	digits := strconv.FormatInt(cents/100, 10)
	var groups []string
	for len(digits) > 3 {
		groups = append([]string{digits[len(digits)-3:]}, groups...)
		digits = digits[:len(digits)-3]
	}
	groups = append([]string{digits}, groups...)
	out := fmt.Sprintf("R$ %s,%02d", strings.Join(groups, "."), cents%100)
	if v < 0 && cents > 0 {
		out = "-" + out
	}
	return out
}

// WeeklyDigest monta o resumo semanal: o que merece atenção agora, calculado só por código (sem IA).
func (i *Insights) WeeklyDigest(ctx context.Context, today time.Time) (string, error) {
	month := MonthStart(today)
	totals, err := i.reader.MonthlyTotals(ctx, month, month)
	if err != nil || len(totals) != 1 {
		return "", fmt.Errorf("totais do mês indisponíveis: %w", err)
	}
	review, err := i.Review(ctx, month)
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
	return FormatDigest(today, totals[0], review, goals, savings), nil
}

// FormatDigest escreve o resumo em texto simples (sem Markdown, já que nomes de contas vêm do banco).
func FormatDigest(today time.Time, totals ports.MonthTotals, review Review, goals []GoalProgress, savings []DecisionResult) string {
	var alerts []string
	add := func(format string, args ...any) { alerts = append(alerts, "• "+fmt.Sprintf(format, args...)) }

	for _, d := range savings {
		if d.Status == SavingReturned {
			add("A cobrança voltou: %s (você tinha marcado como cancelada; foram %s neste mês).", cleanDigestLabel(d.Decision.Label), FormatBRL(d.Returned))
		}
	}
	newCount := 0
	for _, c := range review.Candidates {
		if c.Dismissed {
			continue
		}
		switch {
		case c.Kind == ReviewDuplicate:
			add("Possível cobrança duplicada: %s (%d vezes de %s em poucos dias).", cleanDigestLabel(c.Label), c.Count, FormatBRL(c.Amount))
		case c.Kind == ReviewNew && newCount < maxDigestNew:
			newCount++
			add("Conta nova neste mês: %s (%s).", cleanDigestLabel(c.Label), FormatBRL(c.Amount))
		}
	}
	for _, g := range goals {
		switch g.Goal.Kind {
		case ports.GoalCut:
			if g.Current > g.Target {
				add("Meta \"%s\": o mês já passou %s do teto de %s.", g.Goal.Name, FormatBRL(g.Current-g.Target), FormatBRL(g.Target))
			} else if g.Projected != nil && *g.Projected > g.Target {
				add("Meta \"%s\": no ritmo atual o mês fecha em %s, acima do teto de %s.", g.Goal.Name, FormatBRL(*g.Projected), FormatBRL(g.Target))
			}
		case ports.GoalSave:
			if !g.Done && g.Fits != nil && !*g.Fits {
				add("Meta \"%s\": guardar %s por mês não cabe na sobra projetada de %s.", g.Goal.Name, FormatBRL(g.PerMonth), FormatBRL(max(0, *g.Surplus)))
			}
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Resumo semanal — %s\n\n", today.Format("02/01/2006"))
	fmt.Fprintf(&b, "Mês até agora: despesas %s · receitas %s\n", FormatBRL(totals.Expense), FormatBRL(totals.Income))
	if realized, perMonth := SavingsTotals(savings); realized > 0 {
		fmt.Fprintf(&b, "Economia realizada com o que você cancelou: %s (%s por mês).\n", FormatBRL(realized), FormatBRL(perMonth))
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
	label = strings.TrimSpace(installmentRE.ReplaceAllString(label, ""))
	if r := []rune(label); len(r) > 60 {
		label = string(r[:60])
	}
	return label
}
