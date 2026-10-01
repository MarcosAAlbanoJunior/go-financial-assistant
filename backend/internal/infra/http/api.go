package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

const (
	maxSeriesMonths = 60
	defaultPageSize = 50
	maxPageSize     = 100
	maxSearchLen    = 100
	maxLoginBody    = 1 << 10
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
	sess, err := newSessions(password)
	if err != nil {
		return err
	}
	a := &api{reader: reader, sessions: sess, logger: s.logger, now: time.Now}

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
	s.mux.Handle("GET /api/transactions", protected(a.transactions))
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

func (a *api) transactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := ports.TransactionFilter{Limit: defaultPageSize, Search: q.Get("q")}

	month, ok := a.monthParam(w, r, "month", false)
	if !ok {
		return
	}
	if !month.IsZero() {
		f.Month = &month
	}
	if f.Kind, ok = enumParam(w, q.Get("kind"), "kind", kinds); !ok {
		return
	}
	if f.Category, ok = enumParam(w, q.Get("category"), "category", categories); !ok {
		return
	}
	if f.PaymentMethod, ok = enumParam(w, q.Get("payment_method"), "payment_method", paymentMethods); !ok {
		return
	}
	if raw := q.Get("account"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "account inválido")
			return
		}
		f.AccountID = &id
	}
	if utf8.RuneCountInString(f.Search) > maxSearchLen {
		writeError(w, http.StatusBadRequest, "q muito longo")
		return
	}
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
