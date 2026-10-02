package httpserver

import (
	"context"
	"encoding/json"
	"mime"
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
	// A projeção usada nas metas vai até onde as parcelas conhecidas costumam chegar.
	goalProjectionMonths = 24
)

// cutCategories são as categorias de despesa que aceitam meta de redução.
var cutCategories = []domain.Category{domain.CategoryFood, domain.CategoryTransport, domain.CategoryHealth, domain.CategoryEntertainment, domain.CategoryShopping, domain.CategoryMarket, domain.CategoryHousing, domain.CategoryBills, domain.CategoryEducation, domain.CategoryPeople, domain.CategoryOther}

func (a *api) monthStart() time.Time {
	now := a.now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// goalProgress calcula o andamento de todas as metas agora, e o patrimônio (contas correntes + investimentos).
func (a *api) goalProgress(ctx context.Context) ([]usecase.GoalProgress, float64, error) {
	now := a.monthStart()
	goals, err := a.reader.Goals(ctx)
	if err != nil {
		return nil, 0, err
	}
	accounts, err := a.reader.Accounts(ctx)
	if err != nil {
		return nil, 0, err
	}
	positions, err := a.reader.Positions(ctx)
	if err != nil {
		return nil, 0, err
	}
	proj, err := a.buildProjection(ctx, now, goalProjectionMonths)
	if err != nil {
		return nil, 0, err
	}
	var cats []ports.CategoryMonth
	if slices.ContainsFunc(goals, func(g ports.Goal) bool { return g.Kind == ports.GoalCut }) {
		if cats, err = a.reader.CategoryMonths(ctx, now.AddDate(0, -11, 0), now); err != nil {
			return nil, 0, err
		}
	}

	var wealth float64
	if bank := bankBalance(accounts); bank != nil {
		wealth = *bank
	}
	for _, p := range positions {
		wealth += p.Balance
	}
	out := make([]usecase.GoalProgress, len(goals))
	for i, g := range goals {
		out[i] = usecase.BuildGoalProgress(g, wealth, proj, cats, a.now().UTC())
	}
	return out, wealth, nil
}

// goals: as metas com o andamento calculado agora: patrimônio (contas correntes + investimentos),
// projeção e gastos por categoria.
func (a *api) goals(w http.ResponseWriter, r *http.Request) {
	goals, wealth, err := a.goalProgress(r.Context())
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
func (a *api) createGoal(w http.ResponseWriter, r *http.Request) {
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" || !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "requisição não permitida")
		return
	}
	var body struct {
		Kind          string  `json:"kind"`
		Name          string  `json:"name"`
		TargetAmount  float64 `json:"targetAmount"`
		TargetDate    string  `json:"targetDate"`
		Category      string  `json:"category"`
		CutPercent    int     `json:"cutPercent"`
		ReserveMonths int     `json:"reserveMonths"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxGoalBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	name := strings.TrimSpace(body.Name)
	if n := utf8.RuneCountInString(name); n < 1 || n > maxGoalNameLen || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		writeError(w, http.StatusBadRequest, "name deve ter de 1 a 60 caracteres, sem caracteres de controle")
		return
	}
	g := ports.Goal{ID: uuid.New(), Kind: ports.GoalKind(body.Kind), Name: name}
	now := a.monthStart()

	switch g.Kind {
	case ports.GoalSave:
		date, err := time.Parse("2006-01", body.TargetDate)
		if err != nil || !date.After(now) || date.After(now.AddDate(maxGoalYears, 0, 0)) {
			writeError(w, http.StatusBadRequest, "targetDate deve ser um mês futuro (AAAA-MM) em até 10 anos")
			return
		}
		if !(body.TargetAmount > 0 && body.TargetAmount <= maxGoalAmount) {
			writeError(w, http.StatusBadRequest, "targetAmount deve ser maior que zero")
			return
		}
		g.TargetAmount, g.TargetDate = float64(int64(body.TargetAmount*100+0.5))/100, date
	case ports.GoalCut:
		if !slices.Contains(cutCategories, domain.Category(body.Category)) {
			writeError(w, http.StatusBadRequest, "category inválida")
			return
		}
		if body.CutPercent < 1 || body.CutPercent > maxCutPercent {
			writeError(w, http.StatusBadRequest, "cutPercent deve ser de 1 a 90")
			return
		}
		cats, err := a.reader.CategoryMonths(r.Context(), now.AddDate(0, -3, 0), now.AddDate(0, -1, 0))
		if err != nil {
			a.fail(w, "meta", err)
			return
		}
		baseline, ok := usecase.CutBaseline(cats, body.Category, now)
		if !ok {
			writeError(w, http.StatusUnprocessableEntity, "a categoria não tem gastos nos meses anteriores para servir de base")
			return
		}
		g.Category, g.CutPercent, g.Baseline = body.Category, body.CutPercent, baseline
	case ports.GoalReserve:
		if body.ReserveMonths < 1 || body.ReserveMonths > maxReserveMonths {
			writeError(w, http.StatusBadRequest, "reserveMonths deve ser de 1 a 36")
			return
		}
		g.ReserveMonths = body.ReserveMonths
	default:
		writeError(w, http.StatusBadRequest, "kind deve ser SAVE, CUT ou RESERVE")
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

func (a *api) deleteGoal(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "requisição não permitida")
		return
	}
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
