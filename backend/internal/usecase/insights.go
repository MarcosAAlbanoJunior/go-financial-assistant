package usecase

import (
	"context"
	"slices"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// goalProjectionMonths é até onde vai a projeção usada nas metas.
const goalProjectionMonths = 24

// Insights reúne as leituras que alimentam a revisão, as metas, a economia realizada, o Coach e o resumo
// semanal: busca os dados no banco e roda os cálculos. Tanto a API quanto o agendador de resumos usam.
type Insights struct {
	reader ports.DashboardReader
}

func NewInsights(reader ports.DashboardReader) *Insights { return &Insights{reader: reader} }

// MonthStart é o primeiro dia do mês de t, em UTC.
func MonthStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// BankBalance soma o saldo das contas correntes; nil quando não há conta sincronizada ("em conta" é desconhecido, não zero).
func BankBalance(accounts []ports.Account) *float64 {
	var bank *float64
	for _, acc := range accounts {
		if acc.Type == domain.AccountBank {
			sum := acc.Balance
			if bank != nil {
				sum += *bank
			}
			bank = &sum
		}
	}
	return bank
}

// Projection projeta `months` meses a partir de start (primeiro dia do mês atual).
func (i *Insights) Projection(ctx context.Context, start time.Time, months int) (Projection, error) {
	from, to := start.AddDate(0, -BudgetMonths, 0), start.AddDate(0, -1, 0)
	rows, err := i.reader.ExpenseKeyMonths(ctx, from, to)
	if err != nil {
		return Projection{}, err
	}
	rules, err := i.reader.ExpenseRules(ctx)
	if err != nil {
		return Projection{}, err
	}
	incomes, err := i.reader.IncomePayments(ctx, from, to)
	if err != nil {
		return Projection{}, err
	}
	known, err := i.reader.KnownInstallments(ctx, start, start.AddDate(0, months-1, 0))
	if err != nil {
		return Projection{}, err
	}
	return BuildProjection(rows, rules, incomes, known, start, months), nil
}

// Review roda os detectores da revisão sobre o mês to (primeiro dia).
func (i *Insights) Review(ctx context.Context, to time.Time) (Review, error) {
	cats, err := i.reader.CategoryMonths(ctx, to.AddDate(0, -(ReviewMatrixMonths-1), 0), to)
	if err != nil {
		return Review{}, err
	}
	keyRows, err := i.reader.ExpenseKeyMonths(ctx, to.AddDate(0, -(BudgetMonths-1), 0), to)
	if err != nil {
		return Review{}, err
	}
	rules, err := i.reader.ExpenseRules(ctx)
	if err != nil {
		return Review{}, err
	}
	payments, err := i.reader.ExpensePayments(ctx, to, to)
	if err != nil {
		return Review{}, err
	}
	dismissed, err := i.reader.Dismissals(ctx)
	if err != nil {
		return Review{}, err
	}
	return BuildReview(cats, keyRows, rules, payments, dismissed, to), nil
}

// Goals calcula o andamento de todas as metas hoje e o patrimônio (contas correntes + investimentos).
func (i *Insights) Goals(ctx context.Context, today time.Time) ([]GoalProgress, float64, error) {
	now := MonthStart(today)
	goals, err := i.reader.Goals(ctx)
	if err != nil {
		return nil, 0, err
	}
	accounts, err := i.reader.Accounts(ctx)
	if err != nil {
		return nil, 0, err
	}
	positions, err := i.reader.Positions(ctx)
	if err != nil {
		return nil, 0, err
	}
	proj, err := i.Projection(ctx, now, goalProjectionMonths)
	if err != nil {
		return nil, 0, err
	}
	var cats []ports.CategoryMonth
	if slices.ContainsFunc(goals, func(g ports.Goal) bool { return g.Kind == ports.GoalCut }) {
		if cats, err = i.reader.CategoryMonths(ctx, now.AddDate(0, -11, 0), now); err != nil {
			return nil, 0, err
		}
	}

	var wealth float64
	if bank := BankBalance(accounts); bank != nil {
		wealth = *bank
	}
	for _, p := range positions {
		wealth += p.Balance
	}
	out := make([]GoalProgress, len(goals))
	for n, g := range goals {
		out[n] = BuildGoalProgress(g, wealth, proj, cats, today.UTC())
	}
	return out, wealth, nil
}

// Savings confere as decisões "cancelei" contra o que foi cobrado desde então (now = primeiro dia do mês atual).
func (i *Insights) Savings(ctx context.Context, now time.Time) ([]DecisionResult, error) {
	decisions, err := i.reader.Decisions(ctx)
	if err != nil || len(decisions) == 0 {
		return nil, err
	}
	from := slices.MinFunc(decisions, func(x, y ports.Decision) int { return x.Month.Compare(y.Month) }).Month
	rows, err := i.reader.ExpenseKeyMonths(ctx, from, now)
	if err != nil {
		return nil, err
	}
	return BuildSavings(decisions, rows, now), nil
}
