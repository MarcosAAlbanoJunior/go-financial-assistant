package httpserver

import (
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

const budgetMonths = usecase.BudgetMonths

// budget: despesas do mês divididas em fixas, parceladas e variáveis, e a evolução mês a mês.
func (a *api) budget(w http.ResponseWriter, r *http.Request) {
	to, ok := a.monthParam(w, r, "month", true)
	if !ok {
		return
	}
	from := to.AddDate(0, -(budgetMonths - 1), 0)
	rows, err := a.reader.ExpenseKeyMonths(r.Context(), from, to)
	if err != nil {
		a.fail(w, "orçamento", err)
		return
	}
	rules, err := a.reader.ExpenseRules(r.Context())
	if err != nil {
		a.fail(w, "orçamento", err)
		return
	}
	b := usecase.BuildBudget(rows, rules, from, to)

	type month struct {
		Month       string  `json:"month"`
		Fixed       float64 `json:"fixed"`
		Installment float64 `json:"installment"`
		Variable    float64 `json:"variable"`
	}
	type item struct {
		Key           string  `json:"key"`
		Label         string  `json:"label"`
		Category      string  `json:"category"`
		CategoryLabel string  `json:"categoryLabel"`
		Class         string  `json:"class"`
		Manual        bool    `json:"manual"`
		Total         float64 `json:"total"`
		Count         int     `json:"count"`
		Day           int     `json:"day"`
		Paid          bool    `json:"paid"`
		Months        int     `json:"months"`
	}
	series := make([]month, len(b.Series))
	for i, m := range b.Series {
		series[i] = month{formatMonth(m.Month), m.Fixed, m.Installment, m.Variable}
	}
	items := make([]item, len(b.Items))
	for i, it := range b.Items {
		items[i] = item{it.Key, it.Label, it.Category, domain.Category(it.Category).Label(), string(it.Class), it.Manual, it.Total, it.Count, it.Day, it.Paid, it.Months}
	}
	writeJSON(w, http.StatusOK, map[string]any{"month": formatMonth(to), "series": series, "items": items})
}

var projectionRanges = []int{6, 12, 24}

// projection: base da projeção (premissas e parcelas já conhecidas) a partir do mês atual.
// O simulador de financiamento soma o cenário em cima desta base, no navegador.
func (a *api) projection(w http.ResponseWriter, r *http.Request) {
	months := 12
	if raw := r.URL.Query().Get("months"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || !slices.Contains(projectionRanges, n) {
			writeError(w, http.StatusBadRequest, "months deve ser 6, 12 ou 24")
			return
		}
		months = n
	}
	now := a.now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	p, err := a.insights.Projection(r.Context(), start, months)
	if err != nil {
		a.fail(w, "projeção", err)
		return
	}
	a.writeProjection(w, p)
}

func (a *api) writeProjection(w http.ResponseWriter, p usecase.Projection) {
	type month struct {
		Month       string  `json:"month"`
		Fixed       float64 `json:"fixed"`
		Installment float64 `json:"installment"`
		Variable    float64 `json:"variable"`
	}
	months := make([]month, len(p.Months))
	for i, m := range p.Months {
		months[i] = month{formatMonth(m.Month), m.Fixed, m.Installment, m.Variable}
	}
	type source struct {
		Label   string  `json:"label"`
		Monthly float64 `json:"monthly"`
	}
	sources := make([]source, len(p.Assumptions.IncomeSources))
	for i, s := range p.Assumptions.IncomeSources {
		sources[i] = source{s.Label, s.Monthly}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"assumptions": map[string]any{"income": p.Assumptions.Income, "incomeSources": sources, "fixed": p.Assumptions.Fixed, "variable": p.Assumptions.Variable, "basedOn": p.Assumptions.BasedOn},
		"months":      months,
	})
}

var ruleKeyRE = regexp.MustCompile(`^[a-zà-ÿ ]{1,120}$`)

// setExpenseRule corrige à mão se uma conta é fixa ou variável; "AUTO" volta à detecção automática.
// É a única escrita da API (além do login), então exige JSON e mesma origem, como o login.
func (a *api) setExpenseRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key   string `json:"key"`
		Class string `json:"class"`
	}
	if !decodeJSON(w, r, maxRuleBody, &body) {
		return
	}
	var class domain.ExpenseClass
	switch body.Class {
	case "FIXED", "VARIABLE":
		class = domain.ExpenseClass(body.Class)
	case "AUTO":
	default:
		writeError(w, http.StatusBadRequest, "class deve ser FIXED, VARIABLE ou AUTO")
		return
	}
	if !ruleKeyRE.MatchString(body.Key) {
		writeError(w, http.StatusBadRequest, "key inválida")
		return
	}
	if err := a.reader.SetExpenseRule(r.Context(), body.Key, class); err != nil {
		a.fail(w, "regra de despesa", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *api) registerBudget(rt routes) {
	rt.mux.Handle("GET /api/budget", rt.protected(a.budget))
	rt.mux.Handle("GET /api/projection", rt.protected(a.projection))
	rt.mux.Handle("PUT /api/expense-rules", rt.protected(jsonOnly(a.setExpenseRule)))
}
