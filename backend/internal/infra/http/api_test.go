package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

const testPassword = "senha-de-teste-123"

type fakeReader struct {
	monthly        []ports.MonthTotals
	accounts       []ports.Account
	institutions   []ports.Institution
	logos          map[uuid.UUID]fakeLogo
	positions      []ports.Position
	filter         ports.TransactionFilter
	by             ports.BreakdownDimension
	groupBy        ports.GroupBy
	from, to       time.Time
	rules          map[string]ports.ExpenseClass
	ruleKey        string
	known          map[string]time.Time
	ruleClass      ports.ExpenseClass
	catMonths      []ports.CategoryMonth
	uncategorized  []ports.UncategorizedGroup
	rules2         []ports.CategoryRule
	decisions      []ports.Decision
	analyses       []ports.CoachAnalysis
	goals          []ports.Goal
	created        []ports.Goal
	deleted        uuid.UUID
	deleteFound    bool
	catFrom        time.Time
	payFrom        time.Time
	payments       []ports.ExpensePayment
	dismissed      []ports.Dismissal
	setDismissed   ports.Dismissal
	setDismissedTo bool
	err            error
}

func (f *fakeReader) MonthlyTotals(_ context.Context, from, to time.Time) ([]ports.MonthTotals, error) {
	if f.monthly != nil || f.err != nil {
		return f.monthly, f.err
	}
	var out []ports.MonthTotals
	for m := from; !m.After(to); m = m.AddDate(0, 1, 0) {
		out = append(out, ports.MonthTotals{Month: m, Income: 100, Expense: 40})
	}
	return out, nil
}
func (f *fakeReader) InvestmentSeries(_ context.Context, from, _ time.Time) ([]ports.InvestmentMonth, error) {
	return []ports.InvestmentMonth{{Month: from, Applied: 10, Redeemed: 2, Cumulative: 8}}, f.err
}
func (f *fakeReader) ExpenseBreakdown(_ context.Context, _ time.Time, by ports.BreakdownDimension) ([]ports.BreakdownItem, error) {
	f.by = by
	return []ports.BreakdownItem{{Key: "FOOD", Name: "FOOD", Total: 30}, {Key: "", Total: 5}}, f.err
}
func (f *fakeReader) Transactions(_ context.Context, flt ports.TransactionFilter) ([]ports.Transaction, int, error) {
	f.filter = flt
	return []ports.Transaction{{ID: uuid.New(), Date: time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC), Category: "FOOD", PaymentMethod: "PIX", Kind: "EXPENSE", Type: "SINGLE", Status: "PAID", Amount: 12.5, FromOpenFinance: true}}, 1, f.err
}
func (f *fakeReader) Positions(context.Context) ([]ports.Position, error) {
	return f.positions, f.err
}
func (f *fakeReader) PortfolioHistory(_ context.Context, from, to time.Time) ([]ports.PortfolioMonth, error) {
	if f.err != nil {
		return nil, f.err
	}
	v := 1500.5
	est := 1400.0
	return []ports.PortfolioMonth{{Month: from, Balance: &est, Estimated: true}, {Month: to, Balance: &v}}, nil
}
func (f *fakeReader) TransactionGroups(_ context.Context, flt ports.TransactionFilter, by ports.GroupBy) ([]ports.TransactionGroup, error) {
	f.filter, f.groupBy = flt, by
	return []ports.TransactionGroup{{Key: "FOOD", Count: 3, Expense: 90.5}, {Key: "SALARY", Count: 1, Income: 5000}}, f.err
}
func (f *fakeReader) ExpenseKeyMonths(_ context.Context, from, to time.Time) ([]ports.ExpenseKeyMonth, error) {
	f.from, f.to = from, to
	return []ports.ExpenseKeyMonth{
		{Key: "netflix", Label: "NETFLIX 12/09", Category: "ENTERTAINMENT", Month: to.AddDate(0, -1, 0), Total: 44.9, Count: 1, Day: 12, AllPaid: true},
		{Key: "netflix", Label: "NETFLIX 12/10", Category: "ENTERTAINMENT", Month: to.AddDate(0, -2, 0), Total: 44.9, Count: 1, Day: 12, AllPaid: true},
		{Key: "netflix", Label: "NETFLIX 12/11", Category: "ENTERTAINMENT", Month: to, Total: 44.9, Count: 1, Day: 12, AllPaid: false},
	}, f.err
}
func (f *fakeReader) IncomePayments(_ context.Context, from, to time.Time) ([]ports.IncomePayment, error) {
	return []ports.IncomePayment{{Key: "salario", Label: "Salário", Month: to, Amount: 5000}}, f.err
}
func (f *fakeReader) KnownInstallments(_ context.Context, from, to time.Time) (map[time.Time]float64, error) {
	f.known = map[string]time.Time{"from": from, "to": to}
	return map[time.Time]float64{from.AddDate(0, 2, 0): 300}, f.err
}
func (f *fakeReader) ExpenseRules(context.Context) (map[string]ports.ExpenseClass, error) {
	return f.rules, f.err
}
func (f *fakeReader) SetExpenseRule(_ context.Context, key string, class ports.ExpenseClass) error {
	f.ruleKey, f.ruleClass = key, class
	return f.err
}
func (f *fakeReader) CategoryMonths(_ context.Context, from, to time.Time) ([]ports.CategoryMonth, error) {
	f.catFrom = from
	return f.catMonths, f.err
}
func (f *fakeReader) ExpensePayments(_ context.Context, from, _ time.Time) ([]ports.ExpensePayment, error) {
	f.payFrom = from
	return f.payments, f.err
}
func (f *fakeReader) Dismissals(context.Context) ([]ports.Dismissal, error) {
	return f.dismissed, f.err
}
func (f *fakeReader) SetDismissal(_ context.Context, d ports.Dismissal, dismissed bool) error {
	f.setDismissed, f.setDismissedTo = d, dismissed
	return f.err
}
func (f *fakeReader) Goals(context.Context) ([]ports.Goal, error) { return f.goals, f.err }
func (f *fakeReader) CreateGoal(_ context.Context, g ports.Goal) error {
	f.created = append(f.created, g)
	return f.err
}
func (f *fakeReader) DeleteGoal(_ context.Context, id uuid.UUID) (bool, error) {
	f.deleted = id
	return f.deleteFound, f.err
}
func (f *fakeReader) SaveCoachAnalysis(_ context.Context, a ports.CoachAnalysis) error {
	f.analyses = append([]ports.CoachAnalysis{a}, f.analyses...)
	return f.err
}
func (f *fakeReader) CoachAnalyses(_ context.Context, month *time.Time, limit int) ([]ports.CoachAnalysis, error) {
	var out []ports.CoachAnalysis
	for _, a := range f.analyses {
		if (month == nil || a.Month.Equal(*month)) && len(out) < limit {
			out = append(out, a)
		}
	}
	return out, f.err
}
func (f *fakeReader) SetCoachAnswer(_ context.Context, id uuid.UUID, key, answer string) (bool, error) {
	for i, a := range f.analyses {
		if a.ID == id {
			if f.analyses[i].Answers == nil {
				f.analyses[i].Answers = map[string]string{}
			}
			if answer == "" {
				delete(f.analyses[i].Answers, key)
			} else {
				f.analyses[i].Answers[key] = answer
			}
			return true, f.err
		}
	}
	return false, f.err
}
func (f *fakeReader) DeleteCoachAnalysis(_ context.Context, id uuid.UUID) (bool, error) {
	for i, a := range f.analyses {
		if a.ID == id {
			f.analyses = append(f.analyses[:i], f.analyses[i+1:]...)
			return true, f.err
		}
	}
	return false, f.err
}
func (f *fakeReader) Decisions(context.Context) ([]ports.Decision, error) { return f.decisions, f.err }
func (f *fakeReader) SetDecision(_ context.Context, d ports.Decision) error {
	f.decisions = append(f.decisions, d)
	f.dismissed = append(f.dismissed, ports.Dismissal{Kind: d.Kind, Key: d.Key})
	return f.err
}
func (f *fakeReader) DeleteDecision(_ context.Context, kind, key string) error {
	f.decisions = slices.DeleteFunc(f.decisions, func(d ports.Decision) bool { return d.Kind == kind && d.Key == key })
	f.dismissed = slices.DeleteFunc(f.dismissed, func(d ports.Dismissal) bool { return d.Kind == kind && d.Key == key })
	return f.err
}
func (f *fakeReader) UncategorizedExpenses(_ context.Context, limit int) ([]ports.UncategorizedGroup, error) {
	return f.uncategorized[:min(limit, len(f.uncategorized))], f.err
}
func (f *fakeReader) SetCategoryRule(_ context.Context, key, category string) (int64, error) {
	f.rules2 = append(f.rules2, ports.CategoryRule{Key: key, Category: category})
	return 3, f.err
}
func (f *fakeReader) Accounts(context.Context) ([]ports.Account, error) { return f.accounts, f.err }
func (f *fakeReader) Institutions(context.Context) ([]ports.Institution, error) {
	return f.institutions, f.err
}
func (f *fakeReader) InstitutionLogo(_ context.Context, id uuid.UUID) ([]byte, string, bool, error) {
	l, ok := f.logos[id]
	return l.data, l.mime, ok, f.err
}

func newTestAPI(t *testing.T, reader *fakeReader) *Server {
	t.Helper()
	s := NewServer(0, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := s.MountAPI(testPassword, reader); err != nil {
		t.Fatal(err)
	}
	return s
}

func do(s *Server, method, path, body string, hdr map[string]string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "203.0.113.9:4000"
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	return rec
}

func login(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	rec := do(s, "POST", "/api/login", `{"password":"`+testPassword+`"}`, map[string]string{"Content-Type": "application/json"})
	if rec.Code != 200 {
		t.Fatalf("login = %d: %s", rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatal("login sem cookie de sessão")
	return nil
}

func TestAPI_RequiresSession(t *testing.T) {
	s := newTestAPI(t, &fakeReader{})
	for _, path := range []string{"/api/me", "/api/summary?month=2026-09", "/api/timeseries", "/api/breakdown?month=2026-09&by=category", "/api/investments", "/api/transactions", "/api/accounts", "/api/portfolio"} {
		if rec := do(s, "GET", path, "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s sem sessão = %d, esperava 401", path, rec.Code)
		}
	}
	forged := &http.Cookie{Name: sessionCookie, Value: "9999999999.deadbeef"}
	if rec := do(s, "GET", "/api/me", "", nil, forged); rec.Code != http.StatusUnauthorized {
		t.Errorf("cookie forjado = %d", rec.Code)
	}
}

func TestAPI_LoginCookieAttributes(t *testing.T) {
	s := newTestAPI(t, &fakeReader{})
	c := login(t, s)
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.Secure {
		t.Errorf("atributos inesperados em HTTP puro: %+v", c)
	}

	// Atrás do nginx (rede privada) com HTTPS, o cookie ganha Secure.
	rec := do(s, "POST", "/api/login", `{"password":"`+testPassword+`"}`, map[string]string{"Content-Type": "application/json", "X-Forwarded-Proto": "https"})
	if rec.Result().Cookies()[0].Secure {
		t.Error("X-Forwarded-Proto de IP público não pode ser confiado")
	}
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"password":"`+testPassword+`"}`))
	req.RemoteAddr = "172.18.0.5:1234"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if cs := rr.Result().Cookies(); len(cs) == 0 || !cs[0].Secure {
		t.Error("cookie deveria ser Secure atrás do proxy HTTPS")
	}

	if rec := do(s, "GET", "/api/me", "", nil, c); rec.Code != 200 {
		t.Errorf("sessão válida = %d", rec.Code)
	}
}

func TestAPI_LoginRejections(t *testing.T) {
	json := map[string]string{"Content-Type": "application/json"}
	for name, tc := range map[string]struct {
		body string
		hdr  map[string]string
		want int
	}{
		"senha errada":     {`{"password":"errada"}`, json, 401},
		"sem senha":        {`{}`, json, 401},
		"corpo inválido":   {`nao-json`, json, 400},
		"form (CSRF)":      {`password=` + testPassword, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, 403},
		"origem diferente": {`{"password":"` + testPassword + `"}`, map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, 403},
		"mesma origem":     {`{"password":"` + testPassword + `"}`, map[string]string{"Content-Type": "application/json", "Origin": "http://example.com"}, 200},
	} {
		s := newTestAPI(t, &fakeReader{}) // um servidor por caso: o rate limit do login é por IP
		if rec := do(s, "POST", "/api/login", tc.body, tc.hdr); rec.Code != tc.want {
			t.Errorf("%s: %d, esperava %d", name, rec.Code, tc.want)
		}
	}
}

func TestAPI_LoginRateLimit(t *testing.T) {
	s := newTestAPI(t, &fakeReader{})
	hdr := map[string]string{"Content-Type": "application/json"}
	var last int
	for i := 0; i < 6; i++ {
		last = do(s, "POST", "/api/login", `{"password":"errada"}`, hdr).Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("a 6ª tentativa deveria ser bloqueada, got %d", last)
	}
}

func TestAPI_TamperedSessionRejected(t *testing.T) {
	s := newTestAPI(t, &fakeReader{})
	c := login(t, s)
	// Mesmo adulterando a data de validade para o futuro, a assinatura não confere.
	c.Value = "9999999999." + strings.SplitN(c.Value, ".", 2)[1]
	if rec := do(s, "GET", "/api/me", "", nil, c); rec.Code != 401 {
		t.Errorf("sessão vencida/inválida = %d", rec.Code)
	}
}

func TestAPI_Logout(t *testing.T) {
	s := newTestAPI(t, &fakeReader{})
	c := login(t, s)
	rec := do(s, "POST", "/api/logout", "", nil, c)
	if rec.Code != 200 || rec.Result().Cookies()[0].MaxAge >= 0 {
		t.Errorf("logout deveria expirar o cookie: %d %+v", rec.Code, rec.Result().Cookies())
	}
	if rec := do(s, "POST", "/api/logout", "", map[string]string{"Origin": "https://evil.example"}, c); rec.Code != 403 {
		t.Errorf("logout cross-origin = %d", rec.Code)
	}
}

func TestAPI_Summary(t *testing.T) {
	r := &fakeReader{accounts: []ports.Account{{Type: "BANK", Balance: 100}, {Type: "BANK", Balance: 50.5}, {Type: "CREDIT", Balance: 999}}}
	s := newTestAPI(t, r)
	c := login(t, s)

	rec := do(s, "GET", "/api/summary?month=2026-09", "", nil, c)
	body := rec.Body.String()
	for _, want := range []string{`"month":"2026-09"`, `"previous":{"month":"2026-08"`, `"bankBalance":150.5`} {
		if rec.Code != 200 || !strings.Contains(body, want) {
			t.Fatalf("falta %s em %d %s", want, rec.Code, body)
		}
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("respostas financeiras não podem ser cacheadas")
	}

	s2 := newTestAPI(t, &fakeReader{})
	rec = do(s2, "GET", "/api/summary?month=2026-09", "", nil, login(t, s2))
	if !strings.Contains(rec.Body.String(), `"bankBalance":null`) {
		t.Errorf("sem contas o saldo deve ser null: %s", rec.Body)
	}
}

func TestAPI_ParameterValidation(t *testing.T) {
	s := newTestAPI(t, &fakeReader{})
	c := login(t, s)
	for _, path := range []string{
		"/api/summary", "/api/summary?month=2026-13", "/api/summary?month=setembro",
		"/api/breakdown?month=2026-09", "/api/breakdown?month=2026-09&by=1%3BDROP%20TABLE%20payments",
		"/api/timeseries?from=2020-01&to=2026-09", "/api/timeseries?from=2026-09&to=2026-01", "/api/timeseries?from=x",
		"/api/transactions?kind=HACK", "/api/transactions?category=x", "/api/transactions?payment_method=x",
		"/api/transactions?account=nao-uuid", "/api/transactions?page=0", "/api/transactions?page=abc",
		"/api/transactions?limit=1000", "/api/transactions?month=2026-9x", "/api/transactions?q=" + strings.Repeat("a", 101),
	} {
		if rec := do(s, "GET", path, "", nil, c); rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, esperava 400", path, rec.Code)
		}
	}
}

func TestAPI_TransactionsFilters(t *testing.T) {
	r := &fakeReader{}
	s := newTestAPI(t, r)
	id := uuid.New()
	rec := do(s, "GET", "/api/transactions?month=2026-09&kind=EXPENSE&category=FOOD&payment_method=PIX&account="+id.String()+"&q=pad&page=3&limit=20", "", nil, login(t, s))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	f := r.filter
	if f.Month == nil || f.Month.Format("2006-01") != "2026-09" || f.Kind != "EXPENSE" || f.Category != "FOOD" ||
		f.PaymentMethod != "PIX" || f.AccountID == nil || *f.AccountID != id || f.Search != "pad" || f.Limit != 20 || f.Offset != 40 {
		t.Errorf("filtro mal montado: %+v", f)
	}
	for _, want := range []string{`"total":1`, `"page":3`, `"source":"OPEN_FINANCE"`, `"categoryLabel":"Alimentação"`, `"date":"2026-09-03"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("falta %s em %s", want, rec.Body)
		}
	}
	if strings.Contains(rec.Body.String(), "external") || strings.Contains(rec.Body.String(), "raw") {
		t.Error("a API não deve expor ID externo nem texto bruto")
	}
}

func TestAPI_BreakdownLabels(t *testing.T) {
	r := &fakeReader{}
	s := newTestAPI(t, r)
	c := login(t, s)
	body := do(s, "GET", "/api/breakdown?month=2026-09&by=category", "", nil, c).Body.String()
	if !strings.Contains(body, `"label":"Alimentação"`) {
		t.Errorf("categoria sem rótulo: %s", body)
	}
	body = do(s, "GET", "/api/breakdown?month=2026-09&by=account", "", nil, c).Body.String()
	if r.by != ports.BreakdownByAccount || !strings.Contains(body, `"label":"Sem conta (manual)"`) {
		t.Errorf("gasto sem conta deveria ser rotulado: %s", body)
	}
}

func TestAPI_DefaultRangeAndLimit(t *testing.T) {
	r := &fakeReader{}
	s := newTestAPI(t, r)
	c := login(t, s)
	rec := do(s, "GET", "/api/timeseries", "", nil, c)
	if rec.Code != 200 || strings.Count(rec.Body.String(), `"month"`) != 12 {
		t.Errorf("padrão deveria ter 12 meses: %d %s", rec.Code, rec.Body)
	}
	if rec := do(s, "GET", "/api/timeseries?from=2026-01&to=2026-12", "", nil, c); strings.Count(rec.Body.String(), `"month"`) != 12 {
		t.Errorf("intervalo explícito: %s", rec.Body)
	}
}

func TestAPI_InternalErrorsAreGeneric(t *testing.T) {
	s := newTestAPI(t, &fakeReader{err: errors.New("pq: senha do banco vazou aqui")})
	c := login(t, s)
	for _, path := range []string{"/api/accounts", "/api/transactions", "/api/investments", "/api/summary?month=2026-09"} {
		rec := do(s, "GET", path, "", nil, c)
		if rec.Code != 500 || strings.Contains(rec.Body.String(), "vazou") {
			t.Errorf("%s: %d %s", path, rec.Code, rec.Body)
		}
	}
}

func TestClientIP(t *testing.T) {
	mk := func(remote, realIP string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote
		if realIP != "" {
			r.Header.Set("X-Real-IP", realIP)
		}
		return r
	}
	if got := clientIP(mk("172.18.0.5:1", "198.51.100.7")); got != "198.51.100.7" {
		t.Errorf("atrás do proxy: %s", got)
	}
	if got := clientIP(mk("203.0.113.9:1", "198.51.100.7")); got != "203.0.113.9" {
		t.Errorf("X-Real-IP forjado por cliente público deve ser ignorado: %s", got)
	}
	if got := clientIP(mk("172.18.0.5:1", "lixo")); got != "172.18.0.5" {
		t.Errorf("X-Real-IP inválido: %s", got)
	}
}

func TestSessions_ExpiredCookieRejected(t *testing.T) {
	sess, _ := newSessions(testPassword)
	expired := "1000"
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: expired + "." + sess.sign(expired)})
	if sess.valid(r) {
		t.Error("cookie vencido (mesmo com assinatura correta) deve ser recusado")
	}
	future := "9999999999"
	r = httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: future + "." + sess.sign(future)})
	if !sess.valid(r) {
		t.Error("cookie válido recusado")
	}
}

func TestAPI_Portfolio(t *testing.T) {
	r := &fakeReader{positions: []ports.Position{
		{ID: uuid.New(), Type: "FIXED_INCOME", Subtype: "CDB", Name: "CDB A", Balance: 1000, Amount: 1100},
		{ID: uuid.New(), Type: "MUTUAL_FUND", Name: "Fundo B", Balance: 300.5},
		{ID: uuid.New(), Type: "FIXED_INCOME", Subtype: "LCI", Name: "LCI C", Balance: 200},
		{ID: uuid.New(), Type: "ALGO_NOVO", Name: "D", Balance: 1},
	}}
	s := newTestAPI(t, r)
	c := login(t, s)

	body := do(s, "GET", "/api/portfolio", "", nil, c).Body.String()
	for _, want := range []string{`"total":1501.5`, `"typeLabel":"Renda fixa"`, `{"key":"Renda fixa","label":"Renda fixa","total":1200}`, `"label":"Outros"`} {
		if !strings.Contains(body, want) {
			t.Errorf("falta %s em %s", want, body)
		}
	}
	if strings.Index(body, `"label":"Renda fixa","total":1200`) > strings.Index(body, `"label":"Fundos"`) {
		t.Error("grupos devem vir do maior para o menor")
	}

	empty := newTestAPI(t, &fakeReader{})
	body = do(empty, "GET", "/api/portfolio", "", nil, login(t, empty)).Body.String()
	if !strings.Contains(body, `"positions":[]`) || !strings.Contains(body, `"byType":[]`) || !strings.Contains(body, `"total":0`) {
		t.Errorf("sem posições deve devolver listas vazias, não null: %s", body)
	}
}

func TestAPI_PortfolioHistory(t *testing.T) {
	s := newTestAPI(t, &fakeReader{})
	c := login(t, s)
	body := do(s, "GET", "/api/portfolio/history?from=2026-08&to=2026-09", "", nil, c).Body.String()
	if !strings.Contains(body, `{"month":"2026-08","balance":1400,"estimated":true}`) || !strings.Contains(body, `{"month":"2026-09","balance":1500.5,"estimated":false}`) {
		t.Errorf("histórico inesperado: %s", body)
	}
	if rec := do(s, "GET", "/api/portfolio/history?from=2020-01&to=2026-09", "", nil, c); rec.Code != 400 {
		t.Errorf("janela longa demais = %d", rec.Code)
	}
	if rec := do(s, "GET", "/api/portfolio/history", "", nil); rec.Code != 401 {
		t.Errorf("sem sessão = %d", rec.Code)
	}
}

func TestAPI_TransactionGroups(t *testing.T) {
	r := &fakeReader{}
	s := newTestAPI(t, r)
	c := login(t, s)

	rec := do(s, "GET", "/api/transactions/groups?by=category&month=2026-09&kind=EXPENSE&q=pad", "", nil, c)
	body := rec.Body.String()
	if rec.Code != 200 || r.groupBy != ports.GroupByCategory || r.filter.Kind != "EXPENSE" || r.filter.Search != "pad" || r.filter.Month == nil {
		t.Fatalf("%d %s filtro=%+v", rec.Code, body, r.filter)
	}
	for _, want := range []string{`"key":"FOOD","label":"Alimentação","count":3,"expense":90.5`, `"label":"Salário/Renda"`, `"income":5000`} {
		if !strings.Contains(body, want) {
			t.Errorf("falta %s em %s", want, body)
		}
	}

	body = do(s, "GET", "/api/transactions/groups?by=day&month=2026-09", "", nil, c).Body.String()
	if r.groupBy != ports.GroupByDay || !strings.Contains(body, `"label":"FOOD"`) {
		t.Errorf("por dia mantém a chave como rótulo: %s", body)
	}

	for _, bad := range []string{"", "?by=payment_method", "?by=category&kind=HACK", "?by=category&day=ontem", "?by=category&account=x"} {
		path := "/api/transactions/groups" + bad
		if bad == "" {
			path += "?month=2026-09"
		}
		if rec := do(s, "GET", path, "", nil, c); rec.Code != 400 {
			t.Errorf("%s = %d, esperava 400", path, rec.Code)
		}
	}
	if rec := do(s, "GET", "/api/transactions/groups?by=category", "", nil); rec.Code != 401 {
		t.Errorf("sem sessão = %d", rec.Code)
	}

	// O filtro por dia vale também na lista.
	do(s, "GET", "/api/transactions?day=2026-09-03", "", nil, c)
	if r.filter.Day == nil || r.filter.Day.Format("2006-01-02") != "2026-09-03" {
		t.Errorf("day não chegou ao filtro: %+v", r.filter)
	}
}

func TestAPI_Budget(t *testing.T) {
	r := &fakeReader{}
	s := newTestAPI(t, r)
	c := login(t, s)

	rec := do(s, "GET", "/api/budget?month=2026-11", "", nil, c)
	body := rec.Body.String()
	if rec.Code != 200 || r.to.Format("2006-01") != "2026-11" || r.from.Format("2006-01") != "2025-12" {
		t.Fatalf("janela de 12 meses até o mês pedido: %d %s from=%v to=%v", rec.Code, body, r.from, r.to)
	}
	for _, want := range []string{`"class":"FIXED"`, `"categoryLabel":"Lazer"`, `"months":3`, `"paid":false`, `"fixed":44.9`, `"month":"2026-11"`} {
		if !strings.Contains(body, want) {
			t.Errorf("falta %s em %s", want, body)
		}
	}
	if strings.Count(body, `"installment"`) != 12 {
		t.Errorf("a série deveria ter 12 meses: %s", body)
	}
	if rec := do(s, "GET", "/api/budget", "", nil, c); rec.Code != 400 {
		t.Errorf("sem mês = %d", rec.Code)
	}
	if rec := do(s, "GET", "/api/budget?month=2026-11", "", nil); rec.Code != 401 {
		t.Errorf("sem sessão = %d", rec.Code)
	}
}

func TestAPI_SetExpenseRule(t *testing.T) {
	r := &fakeReader{}
	s := newTestAPI(t, r)
	c := login(t, s)
	json := map[string]string{"Content-Type": "application/json"}

	if rec := do(s, "PUT", "/api/expense-rules", `{"key":"netflix","class":"FIXED"}`, json, c); rec.Code != 200 || r.ruleKey != "netflix" || r.ruleClass != ports.ClassFixed {
		t.Errorf("FIXED: %d %q %q", rec.Code, r.ruleKey, r.ruleClass)
	}
	if rec := do(s, "PUT", "/api/expense-rules", `{"key":"débito mercado","class":"AUTO"}`, json, c); rec.Code != 200 || r.ruleClass != "" {
		t.Errorf("AUTO apaga a regra: %d %q", rec.Code, r.ruleClass)
	}

	for name, tc := range map[string]struct {
		body string
		hdr  map[string]string
		want int
	}{
		"classe inválida":  {`{"key":"netflix","class":"PARCELADA"}`, json, 400},
		"chave com SQL":    {`{"key":"x'; drop table payments;--","class":"FIXED"}`, json, 400},
		"chave maiúscula":  {`{"key":"Netflix","class":"FIXED"}`, json, 400},
		"chave vazia":      {`{"key":"","class":"FIXED"}`, json, 400},
		"chave longa":      {`{"key":"` + strings.Repeat("a", 121) + `","class":"FIXED"}`, json, 400},
		"corpo inválido":   {`nao-json`, json, 400},
		"form (CSRF)":      {`key=netflix&class=FIXED`, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, 403},
		"origem diferente": {`{"key":"netflix","class":"FIXED"}`, map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, 403},
	} {
		if rec := do(s, "PUT", "/api/expense-rules", tc.body, tc.hdr, c); rec.Code != tc.want {
			t.Errorf("%s: %d, esperava %d", name, rec.Code, tc.want)
		}
	}
	if rec := do(s, "PUT", "/api/expense-rules", `{"key":"netflix","class":"FIXED"}`, json); rec.Code != 401 {
		t.Errorf("sem sessão = %d", rec.Code)
	}
}

func TestAPI_Review(t *testing.T) {
	r := &fakeReader{
		catMonths: []ports.CategoryMonth{
			{Category: "FOOD", Month: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Total: 300},
			{Category: "FOOD", Month: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Total: 500},
		},
		payments: []ports.ExpensePayment{
			{Key: "netflix", Label: "NETFLIX", Category: "ENTERTAINMENT", PaymentMethod: "CREDIT_CARD", Date: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC), Amount: 44.9},
			{Key: "netflix", Label: "NETFLIX", Category: "ENTERTAINMENT", PaymentMethod: "CREDIT_CARD", Date: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC), Amount: 44.9},
		},
		dismissed: []ports.Dismissal{{Kind: "FIXED", Key: "netflix"}},
	}
	s := newTestAPI(t, r)
	c := login(t, s)

	rec := do(s, "GET", "/api/review?month=2026-11", "", nil, c)
	body := rec.Body.String()
	if rec.Code != 200 || r.catFrom.Format("2006-01") != "2026-06" || r.payFrom.Format("2006-01") != "2026-11" {
		t.Fatalf("matriz de 6 meses e despesas só do mês pedido: %d %s catFrom=%v payFrom=%v", rec.Code, body, r.catFrom, r.payFrom)
	}
	for _, want := range []string{
		`"months":["2026-06"`, `"categoryLabel":"Alimentação"`, `"values":[0,0,0,0,300,500]`,
		`"kind":"INCREASE"`, `"baseline":300`, // +200 sobre a média do mês anterior
		`"kind":"FIXED"`, `"dismissed":true`, `"annual":538.8`,
		`"kind":"DUPLICATE"`, `"annual":null`, `"count":2`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("falta %s em %s", want, body)
		}
	}
	if rec := do(s, "GET", "/api/review", "", nil, c); rec.Code != 400 {
		t.Errorf("sem mês = %d", rec.Code)
	}
	if rec := do(s, "GET", "/api/review?month=2026-11", "", nil); rec.Code != 401 {
		t.Errorf("sem sessão = %d", rec.Code)
	}
	r.err = errors.New("falha")
	if rec := do(s, "GET", "/api/review?month=2026-11", "", nil, c); rec.Code != 500 || strings.Contains(rec.Body.String(), "falha") {
		t.Errorf("erro interno não vaza detalhe: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAPI_SetDismissal(t *testing.T) {
	r := &fakeReader{}
	s := newTestAPI(t, r)
	c := login(t, s)
	json := map[string]string{"Content-Type": "application/json"}

	if rec := do(s, "PUT", "/api/review-dismissals", `{"kind":"ANT","key":"padaria","dismissed":true}`, json, c); rec.Code != 200 || r.setDismissed != (ports.Dismissal{Kind: "ANT", Key: "padaria"}) || !r.setDismissedTo {
		t.Errorf("dispensar: %d %+v %v", rec.Code, r.setDismissed, r.setDismissedTo)
	}
	if rec := do(s, "PUT", "/api/review-dismissals", `{"kind":"INCREASE","key":"FOOD","dismissed":false}`, json, c); rec.Code != 200 || r.setDismissedTo {
		t.Errorf("restaurar (chave de categoria): %d %v", rec.Code, r.setDismissedTo)
	}

	for name, tc := range map[string]struct {
		body string
		hdr  map[string]string
		want int
	}{
		"tipo inválido":    {`{"kind":"OUTRO","key":"padaria","dismissed":true}`, json, 400},
		"chave com SQL":    {`{"kind":"ANT","key":"x'; drop table payments;--","dismissed":true}`, json, 400},
		"chave vazia":      {`{"kind":"ANT","key":"","dismissed":true}`, json, 400},
		"chave longa":      {`{"kind":"ANT","key":"` + strings.Repeat("a", 121) + `","dismissed":true}`, json, 400},
		"sem dismissed":    {`{"kind":"ANT","key":"padaria"}`, json, 400},
		"corpo inválido":   {`nao-json`, json, 400},
		"form (CSRF)":      {`kind=ANT&key=padaria&dismissed=true`, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, 403},
		"origem diferente": {`{"kind":"ANT","key":"padaria","dismissed":true}`, map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, 403},
	} {
		if rec := do(s, "PUT", "/api/review-dismissals", tc.body, tc.hdr, c); rec.Code != tc.want {
			t.Errorf("%s: %d, esperava %d", name, rec.Code, tc.want)
		}
	}
	if rec := do(s, "PUT", "/api/review-dismissals", `{"kind":"ANT","key":"padaria","dismissed":true}`, json); rec.Code != 401 {
		t.Errorf("sem sessão = %d", rec.Code)
	}
}

func TestAPI_Projection(t *testing.T) {
	r := &fakeReader{}
	s := newTestAPI(t, r)
	s.mux = http.NewServeMux() // monta de novo com relógio fixo
	if err := s.mountAPI("senha-de-teste-123", r, func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) }); err != nil {
		t.Fatal(err)
	}
	c := login(t, s)

	rec := do(s, "GET", "/api/projection?months=6", "", nil, c)
	body := rec.Body.String()
	if rec.Code != 200 || r.known["from"].Format("2006-01") != "2026-10" || r.known["to"].Format("2006-01") != "2027-03" {
		t.Fatalf("seis meses a partir do mês atual: %d %s %v", rec.Code, body, r.known)
	}
	if strings.Count(body, `"installment"`) != 6 || !strings.Contains(body, `"month":"2026-12","fixed":`) || !strings.Contains(body, `"installment":300`) {
		t.Errorf("parcela cadastrada de dez deveria aparecer: %s", body)
	}
	for _, bad := range []string{"months=7", "months=abc", "months=0"} {
		if rec := do(s, "GET", "/api/projection?"+bad, "", nil, c); rec.Code != 400 {
			t.Errorf("%s = %d", bad, rec.Code)
		}
	}
	if rec := do(s, "GET", "/api/projection", "", nil, c); rec.Code != 200 || r.known["to"].Format("2006-01") != "2027-09" {
		t.Errorf("padrão são 12 meses: %d %v", rec.Code, r.known)
	}
	if rec := do(s, "GET", "/api/projection", "", nil); rec.Code != 401 {
		t.Errorf("sem sessão = %d", rec.Code)
	}
}

// newGoalsAPI monta a API com o relógio em 15/10/2026.
func newGoalsAPI(t *testing.T, r *fakeReader) *Server {
	t.Helper()
	s := newTestAPI(t, r)
	s.mux = http.NewServeMux()
	if err := s.mountAPI(testPassword, r, func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) }); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAPI_Goals(t *testing.T) {
	r := &fakeReader{
		accounts:  []ports.Account{{Type: "BANK", Balance: 1000}, {Type: "CREDIT", Balance: 500}},
		positions: []ports.Position{{Balance: 2000}},
		catMonths: []ports.CategoryMonth{{Category: "FOOD", Month: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Total: 550}},
		goals: []ports.Goal{
			{ID: uuid.New(), Kind: ports.GoalSave, Name: "Viagem", TargetAmount: 6000, TargetDate: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), CreatedAt: time.Now()},
			{ID: uuid.New(), Kind: ports.GoalCut, Name: "Comer menos fora", Category: "FOOD", CutPercent: 20, Baseline: 800, CreatedAt: time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)},
			{ID: uuid.New(), Kind: ports.GoalReserve, Name: "Reserva", ReserveMonths: 6, CreatedAt: time.Now()},
		},
	}
	s := newGoalsAPI(t, r)
	c := login(t, s)

	rec := do(s, "GET", "/api/goals", "", nil, c)
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, body)
	}
	for _, want := range []string{
		`"wealth":3000`, // conta corrente + investimentos; o cartão não entra
		`"targetDate":"2027-01"`, `"monthsLeft":3`, `"perMonth":1000`, `"target":6000`,
		`"categoryLabel":"Alimentação"`, `"target":640`, `"current":550`, `"hit":true`, // 20% abaixo de 800
		`"reserveMonths":6`,
		`"dayOfMonth":15`, `"daysInMonth":31`, `"projected":1136.6`, // 550 em 15 dias de 31: fecha em ~1.136
	} {
		if !strings.Contains(body, want) {
			t.Errorf("falta %s em %s", want, body)
		}
	}
	if rec := do(s, "GET", "/api/goals", "", nil); rec.Code != 401 {
		t.Errorf("sem sessão = %d", rec.Code)
	}
}

func TestAPI_CreateGoal(t *testing.T) {
	r := &fakeReader{catMonths: []ports.CategoryMonth{
		{Category: "FOOD", Month: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Total: 800},
		{Category: "FOOD", Month: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Total: 600},
	}}
	s := newGoalsAPI(t, r)
	c := login(t, s)
	json := map[string]string{"Content-Type": "application/json"}
	post := func(body string) int { return do(s, "POST", "/api/goals", body, json, c).Code }

	if code := post(`{"kind":"SAVE","name":"  Viagem  ","targetAmount":12000.456,"targetDate":"2027-06"}`); code != 201 {
		t.Fatalf("SAVE: %d", code)
	}
	if g := r.created[0]; g.Name != "Viagem" || g.TargetAmount != 12000.46 || g.TargetDate.Format("2006-01-02") != "2027-06-01" || g.Kind != ports.GoalSave {
		t.Errorf("SAVE gravada: %+v", g)
	}
	if code := post(`{"kind":"CUT","name":"Comida","category":"FOOD","cutPercent":10}`); code != 201 || r.created[1].Baseline != 700 || r.created[1].Category != "FOOD" {
		t.Errorf("CUT: baseline é a média dos meses anteriores (700): %d %+v", code, r.created)
	}
	if code := post(`{"kind":"RESERVE","name":"Reserva","reserveMonths":6}`); code != 201 || r.created[2].ReserveMonths != 6 {
		t.Errorf("RESERVE: %d %+v", code, r.created)
	}

	for name, body := range map[string]string{
		"tipo inválido":         `{"kind":"X","name":"a"}`,
		"nome vazio":            `{"kind":"RESERVE","name":"   ","reserveMonths":6}`,
		"nome longo":            `{"kind":"RESERVE","name":"` + strings.Repeat("a", 61) + `","reserveMonths":6}`,
		"nome com controle":     `{"kind":"RESERVE","name":"a\u0000b","reserveMonths":6}`,
		"campo desconhecido":    `{"kind":"RESERVE","name":"a","reserveMonths":6,"id":"x"}`,
		"data passada":          `{"kind":"SAVE","name":"a","targetAmount":100,"targetDate":"2026-10"}`,
		"data em 11 anos":       `{"kind":"SAVE","name":"a","targetAmount":100,"targetDate":"2037-11"}`,
		"data inválida":         `{"kind":"SAVE","name":"a","targetAmount":100,"targetDate":"amanhã"}`,
		"valor zero":            `{"kind":"SAVE","name":"a","targetAmount":0,"targetDate":"2027-06"}`,
		"valor enorme":          `{"kind":"SAVE","name":"a","targetAmount":1e12,"targetDate":"2027-06"}`,
		"categoria de renda":    `{"kind":"CUT","name":"a","category":"SALARY","cutPercent":10}`,
		"categoria com SQL":     `{"kind":"CUT","name":"a","category":"FOOD'; drop table goals;--","cutPercent":10}`,
		"percentual alto":       `{"kind":"CUT","name":"a","category":"FOOD","cutPercent":95}`,
		"percentual zero":       `{"kind":"CUT","name":"a","category":"FOOD","cutPercent":0}`,
		"meses de reserva alto": `{"kind":"RESERVE","name":"a","reserveMonths":37}`,
		"corpo inválido":        `nao-json`,
	} {
		if code := post(body); code != 400 {
			t.Errorf("%s: %d, esperava 400", name, code)
		}
	}
	if code := post(`{"kind":"CUT","name":"a","category":"TRANSPORT","cutPercent":10}`); code != 422 {
		t.Errorf("categoria sem histórico = %d", code)
	}
	if code := do(s, "POST", "/api/goals", `kind=RESERVE`, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, c).Code; code != 403 {
		t.Errorf("form (CSRF) = %d", code)
	}
	if code := do(s, "POST", "/api/goals", `{"kind":"RESERVE","name":"a","reserveMonths":6}`, map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, c).Code; code != 403 {
		t.Errorf("origem diferente = %d", code)
	}
	if code := do(s, "POST", "/api/goals", `{"kind":"RESERVE","name":"a","reserveMonths":6}`, json).Code; code != 401 {
		t.Errorf("sem sessão = %d", code)
	}
	if len(r.created) != 3 {
		t.Errorf("só as 3 válidas foram gravadas: %d", len(r.created))
	}

	r.goals = make([]ports.Goal, maxGoals)
	if code := post(`{"kind":"RESERVE","name":"a","reserveMonths":6}`); code != 409 {
		t.Errorf("limite de metas = %d", code)
	}
}

func TestAPI_DeleteGoal(t *testing.T) {
	r := &fakeReader{deleteFound: true}
	s := newGoalsAPI(t, r)
	c := login(t, s)
	id := uuid.New()

	if rec := do(s, "DELETE", "/api/goals/"+id.String(), "", nil, c); rec.Code != 200 || r.deleted != id {
		t.Errorf("apagar: %d %v", rec.Code, r.deleted)
	}
	r.deleteFound = false
	if rec := do(s, "DELETE", "/api/goals/"+id.String(), "", nil, c); rec.Code != 404 {
		t.Errorf("inexistente = %d", rec.Code)
	}
	if rec := do(s, "DELETE", "/api/goals/1;drop", "", nil, c); rec.Code != 400 {
		t.Errorf("id inválido = %d", rec.Code)
	}
	if rec := do(s, "DELETE", "/api/goals/"+id.String(), "", map[string]string{"Origin": "https://evil.example"}, c); rec.Code != 403 {
		t.Errorf("origem diferente = %d", rec.Code)
	}
	if rec := do(s, "DELETE", "/api/goals/"+id.String(), "", nil); rec.Code != 401 {
		t.Errorf("sem sessão = %d", rec.Code)
	}
}

func TestAPI_Savings(t *testing.T) {
	jul := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	r := &fakeReader{decisions: []ports.Decision{
		{Kind: "FIXED", Key: "netflix", Label: "NETFLIX", Category: "ENTERTAINMENT", Month: jul, Monthly: 44.9}, // o fake ainda cobra: voltou
		{Kind: "FIXED", Key: "spotify", Label: "SPOTIFY", Category: "ENTERTAINMENT", Month: jul, Monthly: 20},   // sem cobrança: ago e set confirmados
	}}
	s := newGoalsAPI(t, r)
	c := login(t, s)

	rec := do(s, "GET", "/api/savings", "", nil, c)
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, body)
	}
	for _, want := range []string{`"status":"RETURNED"`, `"returned":44.9`, `"status":"CONFIRMED"`, `"monthsConfirmed":2`, `"realized":40`, `"perMonth":20`, `"perYear":240`, `"categoryLabel":"Lazer"`, `"month":"2026-07"`} {
		if !strings.Contains(body, want) {
			t.Errorf("falta %s em %s", want, body)
		}
	}
	if rec := do(s, "GET", "/api/savings", "", nil); rec.Code != 401 {
		t.Errorf("sem sessão = %d", rec.Code)
	}
	r.decisions = nil
	if rec := do(s, "GET", "/api/savings", "", nil, c); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"decisions":[]`) {
		t.Errorf("sem decisões: %d %s", rec.Code, rec.Body)
	}
}

func TestAPI_SetDecision(t *testing.T) {
	r := &fakeReader{}
	s := newGoalsAPI(t, r)
	c := login(t, s)
	json := map[string]string{"Content-Type": "application/json"}
	put := func(body string) int { return do(s, "PUT", "/api/savings/decisions", body, json, c).Code }

	// O servidor lê nome, categoria e custo da sugestão do mês (a conta fixa do fake).
	if code := put(`{"kind":"FIXED","key":"netflix","month":"2026-10","decided":true}`); code != 200 {
		t.Fatalf("decidir: %d", code)
	}
	if len(r.decisions) != 1 || r.decisions[0].Monthly != 44.9 || r.decisions[0].Label == "" || r.decisions[0].Month.Format("2006-01") != "2026-10" || r.decisions[0].Category != "ENTERTAINMENT" {
		t.Errorf("decisão gravada com os dados do servidor: %+v", r.decisions)
	}
	if code := put(`{"kind":"FIXED","key":"netflix","decided":false}`); code != 200 || len(r.decisions) != 0 {
		t.Errorf("desfazer: %d %+v", code, r.decisions)
	}

	for name, tc := range map[string]struct {
		body string
		want int
	}{
		"tipo avulso":          {`{"kind":"DUPLICATE","key":"netflix","month":"2026-10","decided":true}`, 400},
		"aumento":              {`{"kind":"INCREASE","key":"FOOD","month":"2026-10","decided":true}`, 400},
		"conta nova":           {`{"kind":"NEW","key":"netflix","month":"2026-10","decided":true}`, 400},
		"chave com SQL":        {`{"kind":"FIXED","key":"x'; drop table payments;--","month":"2026-10","decided":true}`, 400},
		"sem decided":          {`{"kind":"FIXED","key":"netflix","month":"2026-10"}`, 400},
		"mês futuro":           {`{"kind":"FIXED","key":"netflix","month":"2026-12","decided":true}`, 400},
		"mês inválido":         {`{"kind":"FIXED","key":"netflix","month":"x","decided":true}`, 400},
		"sugestão inexistente": {`{"kind":"FIXED","key":"conta que nao existe","month":"2026-10","decided":true}`, 404},
		"corpo inválido":       {`nao-json`, 400},
	} {
		if code := put(tc.body); code != tc.want {
			t.Errorf("%s: %d, esperava %d", name, code, tc.want)
		}
	}
	if len(r.decisions) != 0 {
		t.Errorf("nada inválido pode ser gravado: %+v", r.decisions)
	}
	if code := do(s, "PUT", "/api/savings/decisions", `kind=FIXED`, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, c).Code; code != 403 {
		t.Errorf("form (CSRF) = %d", code)
	}
	if code := do(s, "PUT", "/api/savings/decisions", `{"kind":"FIXED","key":"netflix","month":"2026-10","decided":true}`, map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, c).Code; code != 403 {
		t.Errorf("origem diferente = %d", code)
	}
	if code := do(s, "PUT", "/api/savings/decisions", `{"kind":"FIXED","key":"netflix","month":"2026-10","decided":true}`, json).Code; code != 401 {
		t.Errorf("sem sessão = %d", code)
	}
}

type fakeLogo struct {
	data []byte
	mime string
}
