package httpserver

import (
	"net/http"
	"regexp"
	"slices"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

// review: matriz categoria x mês e sugestões de corte do mês, calculadas pelos detectores (sem IA).
func (a *api) review(w http.ResponseWriter, r *http.Request) {
	to, ok := a.monthParam(w, r, "month", true)
	if !ok {
		return
	}
	rev, err := a.insights.Review(r.Context(), to)
	if err != nil {
		a.fail(w, "revisão", err)
		return
	}

	type row struct {
		Category      string    `json:"category"`
		CategoryLabel string    `json:"categoryLabel"`
		Values        []float64 `json:"values"`
	}
	type candidate struct {
		Kind          string   `json:"kind"`
		Key           string   `json:"key"`
		Label         string   `json:"label"`
		Category      string   `json:"category"`
		CategoryLabel string   `json:"categoryLabel"`
		Monthly       float64  `json:"monthly"`
		Annual        *float64 `json:"annual"` // nulo nas avulsas (duplicata, conta nova)
		Amount        float64  `json:"amount"`
		Baseline      float64  `json:"baseline"`
		Count         int      `json:"count"`
		Months        int      `json:"months"`
		Dismissed     bool     `json:"dismissed"`
	}
	months := make([]string, len(rev.Months))
	for i, m := range rev.Months {
		months[i] = formatMonth(m)
	}
	matrix := make([]row, len(rev.Matrix))
	for i, m := range rev.Matrix {
		matrix[i] = row{m.Category, domain.Category(m.Category).Label(), m.Values}
	}
	candidates := make([]candidate, len(rev.Candidates))
	for i, c := range rev.Candidates {
		label := domain.Category(c.Category).Label()
		if c.Label == "" {
			c.Label = label
		}
		out := candidate{c.Kind, c.Key, c.Label, c.Category, label, c.Saving, nil, c.Amount, c.Baseline, c.Count, c.Months, c.Dismissed}
		if c.Recurring {
			annual := c.Saving * 12
			out.Annual = &annual
		}
		candidates[i] = out
	}
	writeJSON(w, http.StatusOK, map[string]any{"month": formatMonth(to), "months": months, "matrix": matrix, "candidates": candidates})
}

var dismissalKeyRE = regexp.MustCompile(`^[A-Za-zà-ÿ_ ]{1,120}$`)

// setDismissal dispensa (ou traz de volta) uma sugestão da revisão. Escrita: exige JSON e mesma origem.
func (a *api) setDismissal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind      string `json:"kind"`
		Key       string `json:"key"`
		Dismissed *bool  `json:"dismissed"`
	}
	if !decodeJSON(w, r, maxRuleBody, &body) {
		return
	}
	if body.Dismissed == nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if !slices.Contains(usecase.ReviewKinds, body.Kind) {
		writeError(w, http.StatusBadRequest, "kind inválido")
		return
	}
	if !dismissalKeyRE.MatchString(body.Key) {
		writeError(w, http.StatusBadRequest, "key inválida")
		return
	}
	if err := a.reader.SetDismissal(r.Context(), domain.Dismissal{Kind: body.Kind, Key: body.Key}, *body.Dismissed); err != nil {
		a.fail(w, "sugestão dispensada", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *api) registerReview(rt routes) {
	rt.mux.Handle("GET /api/review", rt.protected(a.review))
	rt.mux.Handle("PUT /api/review-dismissals", rt.protected(jsonOnly(a.setDismissal)))
}
