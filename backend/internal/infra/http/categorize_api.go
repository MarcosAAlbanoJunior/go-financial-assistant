package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"slices"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

const maxUncategorized = 60

// ruleCategories são as categorias que uma conta pode receber; OTHER significa "manter em Outros".
var ruleCategories = []domain.Category{
	domain.CategoryFood, domain.CategoryMarket, domain.CategoryTransport, domain.CategoryHealth, domain.CategoryEntertainment,
	domain.CategoryShopping, domain.CategoryHousing, domain.CategoryBills, domain.CategoryEducation, domain.CategoryPeople, domain.CategoryOther,
}

// categorizeAI monta o que iria à IA (contas de comércio e serviços, nunca Pix) e o hash que a pessoa confere.
func (a *api) categorizeAI(ctx context.Context) (usecase.CategorizeContext, []byte, string, []domain.UncategorizedGroup, error) {
	groups, err := a.reader.UncategorizedExpenses(ctx, maxUncategorized)
	if err != nil {
		return usecase.CategorizeContext{}, nil, "", nil, err
	}
	c, kept := usecase.BuildCategorizeContext(groups)
	payload, err := json.Marshal(c)
	if err != nil {
		return usecase.CategorizeContext{}, nil, "", nil, err
	}
	sum := sha256.Sum256(payload)
	return c, payload, hex.EncodeToString(sum[:]), kept, nil
}

// categorize: as contas que ficaram em Outros, da maior para a menor, e a prévia do que a IA receberia.
func (a *api) categorize(w http.ResponseWriter, r *http.Request) {
	groups, err := a.reader.UncategorizedExpenses(r.Context(), maxUncategorized)
	if err != nil {
		a.fail(w, "categorias", err)
		return
	}
	_, payload, hash, _, err := a.categorizeAI(r.Context())
	if err != nil {
		a.fail(w, "categorias", err)
		return
	}
	type group struct {
		Key      string  `json:"key"`
		Label    string  `json:"label"`
		Count    int     `json:"count"`
		Total    float64 `json:"total"`
		Last     string  `json:"last"`
		Transfer bool    `json:"transfer"` // Pix/TED: só você sabe do que se trata, e nunca vai à IA
	}
	out := make([]group, len(groups))
	for i, g := range groups {
		out[i] = group{g.Key, g.Label, g.Count, g.Total, formatMonth(g.Last), usecase.IsPersonTransfer(g.Label)}
	}
	type option struct {
		Value string `json:"value"`
		Label string `json:"label"`
	}
	options := make([]option, len(ruleCategories))
	for i, c := range ruleCategories {
		options[i] = option{string(c), c.Label()}
	}
	blocked := ""
	if !a.coach.enabled() {
		blocked = coachBlockedText
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"categories": options, "groups": out,
		"ai": map[string]any{"enabled": a.coach.enabled(), "blockedReason": blocked, "provider": "Google Gemini", "hash": hash, "bytes": len(payload), "context": json.RawMessage(payload)},
	})
}

// setCategoryRule classifica uma conta: reclassifica as despesas dela que estavam em Outros e vale para as próximas.
func (a *api) setCategoryRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key      string `json:"key"`
		Category string `json:"category"`
	}
	if !decodeJSON(w, r, maxRuleBody, &body) {
		return
	}
	if !ruleKeyRE.MatchString(body.Key) {
		writeError(w, http.StatusBadRequest, "key inválida")
		return
	}
	if !slices.Contains(ruleCategories, domain.Category(body.Category)) {
		writeError(w, http.StatusBadRequest, "category inválida")
		return
	}
	changed, err := a.reader.SetCategoryRule(r.Context(), body.Key, body.Category)
	if err != nil {
		a.fail(w, "regra de categoria", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"changed": changed})
}

// suggestCategories pede à IA categorias para as contas da prévia. Nada é gravado: a pessoa revisa e aplica.
// Mesmas travas do Coach: plano pago declarado, só o que a prévia mostrou (hash) e uma análise por vez.
func (a *api) suggestCategories(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Hash string `json:"hash"`
	}
	if !decodeJSON(w, r, maxCoachBody, &body) {
		return
	}
	if !a.coach.enabled() {
		writeError(w, http.StatusForbidden, coachBlockedText)
		return
	}
	if !a.coach.begin() {
		writeError(w, http.StatusTooManyRequests, "já há uma análise em andamento")
		return
	}
	defer a.coach.end()

	_, payload, hash, kept, err := a.categorizeAI(r.Context())
	if err != nil {
		a.fail(w, "categorias", err)
		return
	}
	if body.Hash != hash {
		writeError(w, http.StatusConflict, "os dados mudaram desde a prévia: confira de novo o que será enviado")
		return
	}
	if len(kept) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"suggestions": []usecase.CategoryChoice{}})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), coachTimeout)
	defer cancel()
	raw, err := a.coach.advisor.Categorize(ctx, payload)
	if err != nil {
		a.fail(w, "categorias", err)
		return
	}
	type suggestion struct {
		Key      string `json:"key"`
		Category string `json:"category"`
	}
	choices := usecase.ValidateCategories(raw, kept)
	out := make([]suggestion, len(choices))
	for i, c := range choices {
		out[i] = suggestion{c.Key, c.Category}
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": out})
}

func (a *api) registerCategorize(rt routes) {
	rt.mux.Handle("GET /api/categorize", rt.protected(a.categorize))
	rt.mux.Handle("PUT /api/categorize/rules", rt.protected(jsonOnly(a.setCategoryRule)))
	rt.mux.Handle("POST /api/categorize/suggest", newIPRateLimiter(3, time.Minute).middleware(rt.protected(jsonOnly(a.suggestCategories))))
}
