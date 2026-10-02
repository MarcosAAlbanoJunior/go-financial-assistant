package httpserver

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
	"github.com/google/uuid"
)

const (
	maxGoalBody      = 2 << 10
	maxGoals         = 20
	maxGoalNameLen   = 60
	maxGoalAmount    = 1e9
	maxGoalYears     = 10
	maxReserveMonths = 36
	maxCutPercent    = 90
)

// cutCategories são as categorias de despesa que aceitam meta de redução.
var cutCategories = []domain.Category{domain.CategoryFood, domain.CategoryTransport, domain.CategoryHealth, domain.CategoryEntertainment, domain.CategoryShopping, domain.CategoryMarket, domain.CategoryHousing, domain.CategoryBills, domain.CategoryEducation, domain.CategoryPeople, domain.CategoryOther}

func (a *api) monthStart() time.Time {
	now := a.now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// goals: as metas com o andamento calculado agora: patrimônio (contas correntes + investimentos),
// projeção e gastos por categoria.
func (a *api) goals(w http.ResponseWriter, r *http.Request) {
	goals, wealth, err := a.insights.Goals(r.Context(), a.now())
	if err != nil {
		a.fail(w, "metas", err)
		return
	}

	type cutMonth struct {
		Month string  `json:"month"`
		Total float64 `json:"total"`
		Hit   bool    `json:"hit"`
	}
	type goal struct {
		ID            uuid.UUID  `json:"id"`
		Kind          string     `json:"kind"`
		Name          string     `json:"name"`
		TargetDate    *string    `json:"targetDate"`
		Category      string     `json:"category"`
		CategoryLabel string     `json:"categoryLabel"`
		CutPercent    int        `json:"cutPercent"`
		Baseline      float64    `json:"baseline"`
		ReserveMonths int        `json:"reserveMonths"`
		Current       float64    `json:"current"`
		Target        float64    `json:"target"`
		Done          bool       `json:"done"`
		MonthsLeft    int        `json:"monthsLeft"`
		PerMonth      float64    `json:"perMonth"`
		Surplus       *float64   `json:"surplus"`
		Fits          *bool      `json:"fits"`
		Coverage      float64    `json:"coverage"`
		History       []cutMonth `json:"history"`
		Projected     *float64   `json:"projected"`
		DayOfMonth    int        `json:"dayOfMonth"`
		DaysInMonth   int        `json:"daysInMonth"`
	}
	out := make([]goal, len(goals))
	for i, gp := range goals {
		g := gp.Goal
		o := goal{
			ID: g.ID, Kind: string(g.Kind), Name: g.Name, Category: g.Category, CutPercent: g.CutPercent, Baseline: g.Baseline,
			ReserveMonths: g.ReserveMonths, Current: gp.Current, Target: gp.Target, Done: gp.Done, MonthsLeft: gp.MonthsLeft,
			PerMonth: gp.PerMonth, Surplus: gp.Surplus, Fits: gp.Fits, Coverage: gp.Coverage, History: []cutMonth{}, Projected: gp.Projected, DayOfMonth: gp.DayOfMonth, DaysInMonth: gp.DaysInMonth,
		}
		if g.Category != "" {
			o.CategoryLabel = domain.Category(g.Category).Label()
		}
		if g.Kind == ports.GoalSave {
			d := formatMonth(g.TargetDate)
			o.TargetDate = &d
		}
		for _, h := range gp.History {
			o.History = append(o.History, cutMonth{formatMonth(h.Month), h.Total, h.Hit})
		}
		out[i] = o
	}
	writeJSON(w, http.StatusOK, map[string]any{"wealth": wealth, "goals": out})
}

// createGoal cria uma meta. Escrita: exige JSON e mesma origem. Tudo é validado aqui; o cliente só
// informa os parâmetros, e o servidor calcula a base (média) e a data de criação.
// goalInput é o corpo de POST /api/goals; cada tipo de meta usa só os seus campos.
type goalInput struct {
	Kind          string  `json:"kind"`
	Name          string  `json:"name"`
	TargetAmount  float64 `json:"targetAmount"`
	TargetDate    string  `json:"targetDate"`
	Category      string  `json:"category"`
	CutPercent    int     `json:"cutPercent"`
	ReserveMonths int     `json:"reserveMonths"`
}

func (a *api) createGoal(w http.ResponseWriter, r *http.Request) {
	var in goalInput
	if !decodeJSONStrict(w, r, maxGoalBody, &in) {
		return
	}
	g, err := a.goalFromInput(r.Context(), in)
	if err != nil {
		a.respondErr(w, "meta", err)
		return
	}
	existing, err := a.reader.Goals(r.Context())
	if err != nil {
		a.fail(w, "meta", err)
		return
	}
	if len(existing) >= maxGoals {
		writeError(w, http.StatusConflict, "limite de 20 metas atingido")
		return
	}
	if err := a.reader.CreateGoal(r.Context(), g); err != nil {
		a.fail(w, "meta", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": g.ID.String()})
}

// goalFromInput valida o corpo e monta a meta. Erros de validação voltam como apiError (400/422).
func (a *api) goalFromInput(ctx context.Context, in goalInput) (ports.Goal, error) {
	name := strings.TrimSpace(in.Name)
	if n := utf8.RuneCountInString(name); n < 1 || n > maxGoalNameLen || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return ports.Goal{}, badRequest("name deve ter de 1 a 60 caracteres, sem caracteres de controle")
	}
	g := ports.Goal{ID: uuid.New(), Kind: ports.GoalKind(in.Kind), Name: name}
	now := a.monthStart()

	var err error
	switch g.Kind {
	case ports.GoalSave:
		err = fillSaveGoal(&g, in, now)
	case ports.GoalCut:
		err = a.fillCutGoal(ctx, &g, in, now)
	case ports.GoalReserve:
		err = fillReserveGoal(&g, in)
	default:
		err = badRequest("kind deve ser SAVE, CUT ou RESERVE")
	}
	return g, err
}

func fillSaveGoal(g *ports.Goal, in goalInput, now time.Time) error {
	date, err := time.Parse("2006-01", in.TargetDate)
	if err != nil || !date.After(now) || date.After(now.AddDate(maxGoalYears, 0, 0)) {
		return badRequest("targetDate deve ser um mês futuro (AAAA-MM) em até 10 anos")
	}
	if !(in.TargetAmount > 0 && in.TargetAmount <= maxGoalAmount) {
		return badRequest("targetAmount deve ser maior que zero")
	}
	g.TargetAmount, g.TargetDate = float64(int64(in.TargetAmount*100+0.5))/100, date
	return nil
}

func (a *api) fillCutGoal(ctx context.Context, g *ports.Goal, in goalInput, now time.Time) error {
	if !slices.Contains(cutCategories, domain.Category(in.Category)) {
		return badRequest("category inválida")
	}
	if in.CutPercent < 1 || in.CutPercent > maxCutPercent {
		return badRequest("cutPercent deve ser de 1 a 90")
	}
	cats, err := a.reader.CategoryMonths(ctx, now.AddDate(0, -3, 0), now.AddDate(0, -1, 0))
	if err != nil {
		return err
	}
	baseline, ok := usecase.CutBaseline(cats, in.Category, now)
	if !ok {
		return &apiError{http.StatusUnprocessableEntity, "a categoria não tem gastos nos meses anteriores para servir de base"}
	}
	g.Category, g.CutPercent, g.Baseline = in.Category, in.CutPercent, baseline
	return nil
}

func fillReserveGoal(g *ports.Goal, in goalInput) error {
	if in.ReserveMonths < 1 || in.ReserveMonths > maxReserveMonths {
		return badRequest("reserveMonths deve ser de 1 a 36")
	}
	g.ReserveMonths = in.ReserveMonths
	return nil
}

func (a *api) deleteGoal(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	found, err := a.reader.DeleteGoal(r.Context(), id)
	if err != nil {
		a.fail(w, "meta", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "meta não encontrada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *api) registerGoals(rt routes) {
	rt.mux.Handle("GET /api/goals", rt.protected(a.goals))
	rt.mux.Handle("POST /api/goals", rt.protected(jsonOnly(a.createGoal)))
	rt.mux.Handle("DELETE /api/goals/{id}", rt.protected(sameOriginOnly(a.deleteGoal)))
}
