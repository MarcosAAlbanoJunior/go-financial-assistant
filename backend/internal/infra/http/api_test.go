package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

const testPassword = "senha-de-teste-123"

type fakeReader struct {
	monthly  []ports.MonthTotals
	accounts []ports.Account
	filter   ports.TransactionFilter
	by       ports.BreakdownDimension
	err      error
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
func (f *fakeReader) Accounts(context.Context) ([]ports.Account, error) { return f.accounts, f.err }

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
	for _, path := range []string{"/api/me", "/api/summary?month=2026-09", "/api/timeseries", "/api/breakdown?month=2026-09&by=category", "/api/investments", "/api/transactions", "/api/accounts"} {
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
