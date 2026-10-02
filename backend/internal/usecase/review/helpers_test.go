package review

import (
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

// month é o primeiro dia do mês, em UTC, para montar entradas dos testes.
func month(y int, m time.Month) time.Time { return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC) }

// monthsFrom cria uma linha de despesa por mês, a partir de start (de 2026), com os totais dados.
func monthsFrom(key string, start time.Month, totals ...float64) []domain.ExpenseKeyMonth {
	var rows []domain.ExpenseKeyMonth
	for i, t := range totals {
		rows = append(rows, domain.ExpenseKeyMonth{Key: key, Label: key, Month: month(2026, start+time.Month(i)), Total: t, Count: 1, Day: 10, AllPaid: true})
	}
	return rows
}
