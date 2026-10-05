package coach

import (
	"time"
)

// month é o primeiro dia do mês, em UTC, para montar entradas dos testes.
func month(y int, m time.Month) time.Time { return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC) }
