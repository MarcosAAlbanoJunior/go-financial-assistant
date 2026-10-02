package httpserver

import (
	"net/http"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

type totalsJSON struct {
	Month    string  `json:"month"`
	Income   float64 `json:"income"`
	Expense  float64 `json:"expense"`
	Applied  float64 `json:"applied"`
	Redeemed float64 `json:"redeemed"`
}

func toTotalsJSON(m ports.MonthTotals) totalsJSON {
	return totalsJSON{formatMonth(m.Month), m.Income, m.Expense, m.Applied, m.Redeemed}
}

// summary: totais do mês e do mês anterior, mais o saldo atual das contas correntes ("em conta").
func (a *api) summary(w http.ResponseWriter, r *http.Request) {
	month, ok := a.monthParam(w, r, "month", true)
	if !ok {
		return
	}
	totals, err := a.reader.MonthlyTotals(r.Context(), month.AddDate(0, -1, 0), month)
	if err != nil || len(totals) != 2 {
		a.fail(w, "resumo", err)
		return
	}
	accounts, err := a.reader.Accounts(r.Context())
	if err != nil {
		a.fail(w, "resumo", err)
		return
	}
	bank := usecase.BankBalance(accounts)
	writeJSON(w, http.StatusOK, map[string]any{
		"month": formatMonth(month), "current": toTotalsJSON(totals[1]), "previous": toTotalsJSON(totals[0]), "bankBalance": bank,
	})
}

func (a *api) timeseries(w http.ResponseWriter, r *http.Request) {
	from, to, ok := a.rangeParams(w, r)
	if !ok {
		return
	}
	totals, err := a.reader.MonthlyTotals(r.Context(), from, to)
	if err != nil {
		a.fail(w, "série temporal", err)
		return
	}
	out := make([]totalsJSON, len(totals))
	for i, m := range totals {
		out[i] = toTotalsJSON(m)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) breakdown(w http.ResponseWriter, r *http.Request) {
	month, ok := a.monthParam(w, r, "month", true)
	if !ok {
		return
	}
	by := ports.BreakdownDimension(r.URL.Query().Get("by"))
	if by != ports.BreakdownByCategory && by != ports.BreakdownByPaymentMethod && by != ports.BreakdownByAccount {
		writeError(w, http.StatusBadRequest, "by deve ser category, payment_method ou account")
		return
	}
	items, err := a.reader.ExpenseBreakdown(r.Context(), month, by)
	if err != nil {
		a.fail(w, "gastos agrupados", err)
		return
	}
	type item struct {
		Key   string  `json:"key"`
		Label string  `json:"label"`
		Total float64 `json:"total"`
	}
	out := make([]item, len(items))
	for i, it := range items {
		out[i] = item{it.Key, breakdownLabel(by, it), it.Total}
	}
	writeJSON(w, http.StatusOK, out)
}

func breakdownLabel(by ports.BreakdownDimension, it ports.BreakdownItem) string {
	switch by {
	case ports.BreakdownByCategory:
		return domain.Category(it.Key).Label()
	case ports.BreakdownByPaymentMethod:
		return domain.PaymentMethod(it.Key).Label()
	}
	if it.Name == "" {
		return "Sem conta (manual)"
	}
	return it.Name
}

func (a *api) registerSummary(rt routes) {
	rt.mux.Handle("GET /api/summary", rt.protected(a.summary))
	rt.mux.Handle("GET /api/timeseries", rt.protected(a.timeseries))
	rt.mux.Handle("GET /api/breakdown", rt.protected(a.breakdown))
}
