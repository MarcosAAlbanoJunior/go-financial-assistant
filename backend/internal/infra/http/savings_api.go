package httpserver

import (
	"encoding/json"
	"mime"
	"net/http"
	"slices"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

// decisionKinds são os tipos que aceitam "cancelei": os que já provaram se repetir todo mês. Conta nova e
// duplicata podem ser compras avulsas, e contar a sua ausência como economia seria enganoso.
var decisionKinds = []string{usecase.ReviewFixed, usecase.ReviewAnt}

// savings: a economia já realizada pelas contas que a pessoa disse ter cancelado, conferida mês a mês.
func (a *api) savings(w http.ResponseWriter, r *http.Request) {
	results, err := a.insights.Savings(r.Context(), a.monthStart())
	if err != nil {
		a.fail(w, "economia realizada", err)
		return
	}
	type decision struct {
		Kind            string  `json:"kind"`
		Key             string  `json:"key"`
		Label           string  `json:"label"`
		Category        string  `json:"category"`
		CategoryLabel   string  `json:"categoryLabel"`
		Month           string  `json:"month"`
		Monthly         float64 `json:"monthly"`
		Status          string  `json:"status"`
		MonthsConfirmed int     `json:"monthsConfirmed"`
		Realized        float64 `json:"realized"`
		Returned        float64 `json:"returned"`
	}
	out := make([]decision, len(results))
	for i, res := range results {
		d := res.Decision
		out[i] = decision{d.Kind, d.Key, d.Label, d.Category, domain.Category(d.Category).Label(), formatMonth(d.Month), d.Monthly, res.Status, res.MonthsConfirmed, res.Realized, res.Returned}
	}
	realized, perMonth := usecase.SavingsTotals(results)
	writeJSON(w, http.StatusOK, map[string]any{"realized": realized, "perMonth": perMonth, "perYear": perMonth * 12, "decisions": out})
}

// setDecision registra (ou desfaz) o "cancelei" de uma sugestão. O servidor lê nome, categoria e custo mensal
// da própria sugestão do mês: o cliente só diz qual é.
func (a *api) setDecision(w http.ResponseWriter, r *http.Request) {
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" || !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "requisição não permitida")
		return
	}
	var body struct {
		Kind    string `json:"kind"`
		Key     string `json:"key"`
		Month   string `json:"month"`
		Decided *bool  `json:"decided"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRuleBody)).Decode(&body); err != nil || body.Decided == nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if !slices.Contains(decisionKinds, body.Kind) {
		writeError(w, http.StatusBadRequest, "kind deve ser FIXED ou ANT")
		return
	}
	if !dismissalKeyRE.MatchString(body.Key) {
		writeError(w, http.StatusBadRequest, "key inválida")
		return
	}
	if !*body.Decided {
		if err := a.reader.DeleteDecision(r.Context(), body.Kind, body.Key); err != nil {
			a.fail(w, "decisão", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	month, err := time.Parse("2006-01", body.Month)
	if err != nil || month.After(a.monthStart()) {
		writeError(w, http.StatusBadRequest, "month deve ser um mês até o atual (AAAA-MM)")
		return
	}
	rev, err := a.insights.Review(r.Context(), month)
	if err != nil {
		a.fail(w, "decisão", err)
		return
	}
	i := slices.IndexFunc(rev.Candidates, func(c usecase.Candidate) bool { return c.Kind == body.Kind && c.Key == body.Key })
	if i < 0 || rev.Candidates[i].Amount <= 0 {
		writeError(w, http.StatusNotFound, "sugestão não encontrada neste mês")
		return
	}
	c := rev.Candidates[i]
	if err := a.reader.SetDecision(r.Context(), ports.Decision{Kind: c.Kind, Key: c.Key, Label: c.Label, Category: c.Category, Month: month, Monthly: c.Amount}); err != nil {
		a.fail(w, "decisão", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
