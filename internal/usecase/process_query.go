package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func (uc *AnalyzeExpense) processQuery(ctx context.Context, analysis *ports.ExpenseAnalysis) (*ExpenseOutput, error) {
	targetMonth := resolveQueryMonth(analysis.QueryInfo)

	summaries, err := uc.repo.FindPaymentsByMonth(ctx, targetMonth)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar despesas: %w", err)
	}

	if len(summaries) == 0 {
		return &ExpenseOutput{
			Type:       "QUERY",
			QueryMonth: formatMonthPT(targetMonth),
			QueryEmpty: true,
		}, nil
	}

	var total float64
	categories := make([]CategorySummary, 0, len(summaries))
	for _, s := range summaries {
		total += s.Total
		categories = append(categories, CategorySummary{
			Category: domain.Category(s.Category).Label(),
			Total:    s.Total,
		})
	}

	return &ExpenseOutput{
		Type:            "QUERY",
		QueryMonth:      formatMonthPT(targetMonth),
		QueryTotal:      total,
		QueryCategories: categories,
	}, nil
}

func resolveQueryMonth(info *ports.QueryInfo) time.Time {
	now := time.Now().UTC()
	month, year := int(now.Month()), now.Year()

	if info != nil {
		if info.Month >= 1 && info.Month <= 12 {
			month = info.Month
		}
		if info.Year >= 2000 {
			year = info.Year
		}
	}

	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
}

var ptMonths = [...]string{
	"Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho",
	"Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro",
}

func formatMonthPT(t time.Time) string {
	return fmt.Sprintf("%s %d", ptMonths[t.Month()-1], t.Year())
}
