package httpserver

import (
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
)

func (a *api) investments(w http.ResponseWriter, r *http.Request) {
	from, to, ok := a.rangeParams(w, r)
	if !ok {
		return
	}
	series, err := a.reader.InvestmentSeries(r.Context(), from, to)
	if err != nil {
		a.fail(w, "investimentos", err)
		return
	}
	type item struct {
		Month      string  `json:"month"`
		Applied    float64 `json:"applied"`
		Redeemed   float64 `json:"redeemed"`
		Cumulative float64 `json:"cumulative"`
	}
	out := make([]item, len(series))
	for i, m := range series {
		out[i] = item{formatMonth(m.Month), m.Applied, m.Redeemed, m.Cumulative}
	}
	writeJSON(w, http.StatusOK, out)
}

var positionTypeLabels = map[string]string{
	"FIXED_INCOME": "Renda fixa",
	"EQUITY":       "Renda variável",
	"MUTUAL_FUND":  "Fundos",
	"ETF":          "ETFs",
	"SECURITY":     "Previdência",
	"COE":          "COE",
}

func positionTypeLabel(t string) string {
	if l, ok := positionTypeLabels[t]; ok {
		return l
	}
	return "Outros"
}

// portfolio: saldo real das posições de investimento (Open Finance), total e por tipo.
func (a *api) portfolio(w http.ResponseWriter, r *http.Request) {
	positions, err := a.reader.Positions(r.Context())
	if err != nil {
		a.fail(w, "posições", err)
		return
	}
	type position struct {
		ID        uuid.UUID `json:"id"`
		Type      string    `json:"type"`
		TypeLabel string    `json:"typeLabel"`
		Subtype   string    `json:"subtype"`
		Name      string    `json:"name"`
		Balance   float64   `json:"balance"`
		Amount    float64   `json:"amount"`
		UpdatedAt time.Time `json:"updatedAt"`
	}
	type group struct {
		Key   string  `json:"key"`
		Label string  `json:"label"`
		Total float64 `json:"total"`
	}
	var (
		total   float64
		items   = make([]position, len(positions))
		byType  = []group{}
		typeIdx = map[string]int{}
	)
	for i, p := range positions {
		items[i] = position{p.ID, p.Type, positionTypeLabel(p.Type), p.Subtype, p.Name, p.Balance, p.Amount, p.UpdatedAt}
		total += p.Balance
		label := positionTypeLabel(p.Type)
		if idx, ok := typeIdx[label]; ok {
			byType[idx].Total += p.Balance
		} else {
			typeIdx[label] = len(byType)
			byType = append(byType, group{label, label, p.Balance})
		}
	}
	sort.SliceStable(byType, func(i, j int) bool { return byType[i].Total > byType[j].Total })
	writeJSON(w, http.StatusOK, map[string]any{"total": total, "positions": items, "byType": byType})
}

func (a *api) portfolioHistory(w http.ResponseWriter, r *http.Request) {
	from, to, ok := a.rangeParams(w, r)
	if !ok {
		return
	}
	months, err := a.reader.PortfolioHistory(r.Context(), from, to)
	if err != nil {
		a.fail(w, "histórico do patrimônio", err)
		return
	}
	type item struct {
		Month     string   `json:"month"`
		Balance   *float64 `json:"balance"`
		Estimated bool     `json:"estimated"`
	}
	out := make([]item, len(months))
	for i, m := range months {
		out[i] = item{formatMonth(m.Month), m.Balance, m.Estimated}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) registerInvestments(rt routes) {
	rt.mux.Handle("GET /api/investments", rt.protected(a.investments))
	rt.mux.Handle("GET /api/portfolio", rt.protected(a.portfolio))
	rt.mux.Handle("GET /api/portfolio/history", rt.protected(a.portfolioHistory))
}
