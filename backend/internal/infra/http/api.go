package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
	"github.com/google/uuid"
)

const (
	maxSeriesMonths = 60
	defaultPageSize = 50
	maxPageSize     = 100
	maxSearchLen    = 100
	maxLoginBody    = 1 << 10
	maxRuleBody     = 1 << 10
)

var (
	categories     = []domain.Category{domain.CategoryFood, domain.CategoryTransport, domain.CategoryHealth, domain.CategoryEntertainment, domain.CategoryShopping, domain.CategoryMarket, domain.CategoryInvestment, domain.CategorySalary, domain.CategoryOther}
	paymentMethods = []domain.PaymentMethod{domain.PaymentMethodCash, domain.PaymentMethodCreditCard, domain.PaymentMethodDebitCard, domain.PaymentMethodPix, domain.PaymentMethodOther}
	kinds          = []domain.PurchaseKind{domain.KindExpense, domain.KindIncome, domain.KindTransfer}
)

// api serve as rotas JSON de leitura do dashboard sob /api.
type api struct {
	reader   ports.DashboardReader
	sessions *sessions
	logger   *slog.Logger
	now      func() time.Time
}

// MountAPI registra a API do dashboard. Tudo, exceto o login, exige sessão.
func (s *Server) MountAPI(password string, reader ports.DashboardReader) error {
	return s.mountAPI(password, reader, time.Now)
}

func (s *Server) mountAPI(password string, reader ports.DashboardReader, now func() time.Time) error {
	sess, err := newSessions(password)
	if err != nil {
		return err
	}
	a := &api{reader: reader, sessions: sess, logger: s.logger, now: now}

	// Login com limite apertado contra tentativa de força bruta; o restante, mais folgado.
	loginLimiter := newIPRateLimiter(5, time.Minute)
	apiLimiter := newIPRateLimiter(300, time.Minute)

	protected := func(h http.HandlerFunc) http.Handler {
		return apiLimiter.middleware(sess.require(h))
	}
	s.mux.Handle("POST /api/login", loginLimiter.middleware(http.HandlerFunc(a.login)))
	s.mux.Handle("POST /api/logout", protected(a.logout))
	s.mux.Handle("GET /api/me", protected(func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, map[string]bool{"ok": true}) }))
	s.mux.Handle("GET /api/summary", protected(a.summary))
	s.mux.Handle("GET /api/timeseries", protected(a.timeseries))
	s.mux.Handle("GET /api/breakdown", protected(a.breakdown))
	s.mux.Handle("GET /api/investments", protected(a.investments))
	s.mux.Handle("GET /api/budget", protected(a.budget))
	s.mux.Handle("GET /api/projection", protected(a.projection))
	s.mux.Handle("PUT /api/expense-rules", protected(a.setExpenseRule))
	s.mux.Handle("GET /api/review", protected(a.review))
	s.mux.Handle("PUT /api/review-dismissals", protected(a.setDismissal))
	s.mux.Handle("GET /api/portfolio", protected(a.portfolio))
	s.mux.Handle("GET /api/portfolio/history", protected(a.portfolioHistory))
	s.mux.Handle("GET /api/transactions", protected(a.transactions))
	s.mux.Handle("GET /api/transactions/groups", protected(a.transactionGroups))
	s.mux.Handle("GET /api/accounts", protected(a.accounts))
	return nil
}

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	// JSON obrigatório + mesma origem: um formulário de outro site não consegue fazer login em nome do usuário (CSRF).
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" || !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "requisição não permitida")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBody)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if !a.sessions.checkPassword(body.Password) {
		a.logger.Warn("login do dashboard recusado", "ip", clientIP(r))
		writeError(w, http.StatusUnauthorized, "senha incorreta")
		return
	}
	a.sessions.issue(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *api) logout(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "requisição não permitida")
		return
	}
	a.sessions.clear(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

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
	var bank *float64 // nil quando não há conta sincronizada: "em conta" é desconhecido, não zero
	for _, acc := range accounts {
		if acc.Type == "BANK" {
			sum := acc.Balance
			if bank != nil {
				sum += *bank
			}
			bank = &sum
		}
	}
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

	rows, err := a.reader.ExpenseKeyMonths(r.Context(), start.AddDate(0, -budgetMonths, 0), start.AddDate(0, -1, 0))
	if err == nil {
		var rules map[string]ports.ExpenseClass
		if rules, err = a.reader.ExpenseRules(r.Context()); err == nil {
			var incomes []ports.IncomePayment
			if incomes, err = a.reader.IncomePayments(r.Context(), start.AddDate(0, -budgetMonths, 0), start.AddDate(0, -1, 0)); err == nil {
				var known map[time.Time]float64
				if known, err = a.reader.KnownInstallments(r.Context(), start, start.AddDate(0, months-1, 0)); err == nil {
					a.writeProjection(w, usecase.BuildProjection(rows, rules, incomes, known, start, months))
					return
				}
			}
		}
	}
	a.fail(w, "projeção", err)
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
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" || !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "requisição não permitida")
		return
	}
	var body struct {
		Key   string `json:"key"`
		Class string `json:"class"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRuleBody)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	var class ports.ExpenseClass
	switch body.Class {
	case "FIXED", "VARIABLE":
		class = ports.ExpenseClass(body.Class)
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

// review: matriz categoria x mês e sugestões de corte do mês, calculadas pelos detectores (sem IA).
func (a *api) review(w http.ResponseWriter, r *http.Request) {
	to, ok := a.monthParam(w, r, "month", true)
	if !ok {
		return
	}
	ctx := r.Context()
	matrixFrom := to.AddDate(0, -(usecase.ReviewMatrixMonths - 1), 0)
	cats, err := a.reader.CategoryMonths(ctx, matrixFrom, to)
	if err != nil {
		a.fail(w, "revisão", err)
		return
	}
	keyRows, err := a.reader.ExpenseKeyMonths(ctx, to.AddDate(0, -(budgetMonths-1), 0), to)
	if err != nil {
		a.fail(w, "revisão", err)
		return
	}
	rules, err := a.reader.ExpenseRules(ctx)
	if err != nil {
		a.fail(w, "revisão", err)
		return
	}
	payments, err := a.reader.ExpensePayments(ctx, to, to)
	if err != nil {
		a.fail(w, "revisão", err)
		return
	}
	dismissed, err := a.reader.Dismissals(ctx)
	if err != nil {
		a.fail(w, "revisão", err)
		return
	}
	rev := usecase.BuildReview(cats, keyRows, rules, payments, dismissed, to)

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
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" || !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "requisição não permitida")
		return
	}
	var body struct {
		Kind      string `json:"kind"`
		Key       string `json:"key"`
		Dismissed *bool  `json:"dismissed"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRuleBody)).Decode(&body); err != nil || body.Dismissed == nil {
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
	if err := a.reader.SetDismissal(r.Context(), ports.Dismissal{Kind: body.Kind, Key: body.Key}, *body.Dismissed); err != nil {
		a.fail(w, "sugestão dispensada", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
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

// filterParams lê os filtros comuns da lista e dos grupos de transações, validando cada um.
func (a *api) filterParams(w http.ResponseWriter, r *http.Request) (ports.TransactionFilter, bool) {
	q := r.URL.Query()
	f := ports.TransactionFilter{Limit: defaultPageSize, Search: q.Get("q")}

	month, ok := a.monthParam(w, r, "month", false)
	if !ok {
		return f, false
	}
	if !month.IsZero() {
		f.Month = &month
	}
	if raw := q.Get("day"); raw != "" {
		day, err := time.Parse(time.DateOnly, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "day deve estar no formato AAAA-MM-DD")
			return f, false
		}
		f.Day = &day
	}
	if f.Kind, ok = enumParam(w, q.Get("kind"), "kind", kinds); !ok {
		return f, false
	}
	if f.Category, ok = enumParam(w, q.Get("category"), "category", categories); !ok {
		return f, false
	}
	if f.PaymentMethod, ok = enumParam(w, q.Get("payment_method"), "payment_method", paymentMethods); !ok {
		return f, false
	}
	if raw := q.Get("account"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "account inválido")
			return f, false
		}
		f.AccountID = &id
	}
	if utf8.RuneCountInString(f.Search) > maxSearchLen {
		writeError(w, http.StatusBadRequest, "q muito longo")
		return f, false
	}
	return f, true
}

// transactionGroups: totais da lista de transações por categoria ou por dia, sob os mesmos filtros.
func (a *api) transactionGroups(w http.ResponseWriter, r *http.Request) {
	f, ok := a.filterParams(w, r)
	if !ok {
		return
	}
	by := ports.GroupBy(r.URL.Query().Get("by"))
	if by != ports.GroupByCategory && by != ports.GroupByDay {
		writeError(w, http.StatusBadRequest, "by deve ser category ou day")
		return
	}
	groups, err := a.reader.TransactionGroups(r.Context(), f, by)
	if err != nil {
		a.fail(w, "grupos de transações", err)
		return
	}
	type item struct {
		Key      string  `json:"key"`
		Label    string  `json:"label"`
		Count    int     `json:"count"`
		Expense  float64 `json:"expense"`
		Income   float64 `json:"income"`
		Transfer float64 `json:"transfer"`
	}
	out := make([]item, len(groups))
	for i, g := range groups {
		label := g.Key
		if by == ports.GroupByCategory {
			label = domain.Category(g.Key).Label()
		}
		out[i] = item{g.Key, label, g.Count, g.Expense, g.Income, g.Transfer}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *api) transactions(w http.ResponseWriter, r *http.Request) {
	f, ok := a.filterParams(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	page := 1
	if raw := q.Get("page"); raw != "" {
		var err error
		if page, err = strconv.Atoi(raw); err != nil || page < 1 || page > 10000 {
			writeError(w, http.StatusBadRequest, "page inválido")
			return
		}
	}
	if raw := q.Get("limit"); raw != "" {
		var err error
		if f.Limit, err = strconv.Atoi(raw); err != nil || f.Limit < 1 || f.Limit > maxPageSize {
			writeError(w, http.StatusBadRequest, "limit deve estar entre 1 e 100")
			return
		}
	}
	f.Offset = (page - 1) * f.Limit

	txs, total, err := a.reader.Transactions(r.Context(), f)
	if err != nil {
		a.fail(w, "transações", err)
		return
	}
	type item struct {
		ID                uuid.UUID  `json:"id"`
		Date              string     `json:"date"`
		Description       string     `json:"description"`
		Category          string     `json:"category"`
		CategoryLabel     string     `json:"categoryLabel"`
		PaymentMethod     string     `json:"paymentMethod"`
		PaymentLabel      string     `json:"paymentMethodLabel"`
		Kind              string     `json:"kind"`
		TransferDirection string     `json:"transferDirection,omitempty"`
		Type              string     `json:"type"`
		Status            string     `json:"status"`
		Amount            float64    `json:"amount"`
		InstallmentNumber *int       `json:"installmentNumber,omitempty"`
		AccountID         *uuid.UUID `json:"accountId,omitempty"`
		AccountName       string     `json:"accountName,omitempty"`
		Source            string     `json:"source"`
	}
	items := make([]item, len(txs))
	for i, t := range txs {
		source := "MANUAL"
		if t.FromOpenFinance {
			source = "OPEN_FINANCE"
		}
		items[i] = item{t.ID, t.Date.Format(time.DateOnly), t.Description, t.Category, domain.Category(t.Category).Label(),
			t.PaymentMethod, domain.PaymentMethod(t.PaymentMethod).Label(), t.Kind, t.TransferDirection, t.Type, t.Status,
			t.Amount, t.InstallmentNumber, t.AccountID, t.AccountName, source}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "page": page, "limit": f.Limit})
}

func (a *api) accounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := a.reader.Accounts(r.Context())
	if err != nil {
		a.fail(w, "contas", err)
		return
	}
	type item struct {
		ID                   uuid.UUID `json:"id"`
		Type                 string    `json:"type"`
		Name                 string    `json:"name"`
		Last4                string    `json:"last4"`
		Balance              float64   `json:"balance"`
		CreditLimit          *float64  `json:"creditLimit"`
		AvailableCreditLimit *float64  `json:"availableCreditLimit"`
		UpdatedAt            time.Time `json:"updatedAt"`
	}
	out := make([]item, len(accounts))
	for i, acc := range accounts {
		out[i] = item(acc)
	}
	writeJSON(w, http.StatusOK, out)
}

// monthParam lê um mês "YYYY-MM" e devolve o seu primeiro dia (UTC). Se obrigatório e
// ausente, responde 400; se opcional e ausente, devolve a data zero.
func (a *api) monthParam(w http.ResponseWriter, r *http.Request, name string, required bool) (time.Time, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" && !required {
		return time.Time{}, true
	}
	t, err := time.Parse("2006-01", raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, name+" deve estar no formato AAAA-MM")
		return time.Time{}, false
	}
	return t, true
}

// rangeParams lê from e to (padrão: últimos 12 meses até o mês atual) e limita a janela.
func (a *api) rangeParams(w http.ResponseWriter, r *http.Request) (from, to time.Time, ok bool) {
	now := a.now().UTC()
	current := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	to, from = current, current.AddDate(0, -11, 0)

	q := r.URL.Query()
	if q.Get("to") != "" {
		if to, ok = a.monthParam(w, r, "to", true); !ok {
			return
		}
	}
	if q.Get("from") != "" {
		if from, ok = a.monthParam(w, r, "from", true); !ok {
			return
		}
	} else if q.Get("to") != "" {
		from = to.AddDate(0, -11, 0)
	}
	months := (to.Year()-from.Year())*12 + int(to.Month()-from.Month()) + 1
	if months < 1 || months > maxSeriesMonths {
		writeError(w, http.StatusBadRequest, "o período deve ter de 1 a 60 meses")
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

func enumParam[T ~string](w http.ResponseWriter, value, name string, allowed []T) (string, bool) {
	if value == "" {
		return "", true
	}
	for _, v := range allowed {
		if string(v) == value {
			return value, true
		}
	}
	writeError(w, http.StatusBadRequest, name+" inválido")
	return "", false
}

func formatMonth(t time.Time) string { return t.Format("2006-01") }

// fail registra o erro real no log e devolve ao cliente uma mensagem genérica.
func (a *api) fail(w http.ResponseWriter, what string, err error) {
	if err == nil {
		err = errors.New("resultado inesperado")
	}
	a.logger.Error("erro na API do dashboard", "recurso", what, "error", err)
	writeError(w, http.StatusInternalServerError, "erro interno")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
