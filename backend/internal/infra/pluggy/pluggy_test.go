package pluggy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func tx(typ, category string, amount float64) transaction {
	return transaction{ID: "t1", Date: "2026-09-30T00:00:00.000Z", Description: "Loja", DescriptionRaw: "LOJA SP", Amount: amount, Category: category, Type: typ, Status: "POSTED"}
}

func TestToExternal(t *testing.T) {
	cases := []struct {
		name     string
		account  string
		tx       transaction
		ok       bool
		kind     domain.PurchaseKind
		category domain.Category
		amount   float64
	}{
		{"mercado", "BANK", tx("DEBIT", "Groceries", -42.5), true, domain.KindExpense, domain.CategoryMarket, 42.5},
		{"restaurante", "BANK", tx("DEBIT", "Food and drinks", -30), true, domain.KindExpense, domain.CategoryFood, 30},
		{"sem categoria", "BANK", tx("DEBIT", "", -10), true, domain.KindExpense, domain.CategoryOther, 10},
		{"salário", "BANK", tx("CREDIT", "Salary", 6000), true, domain.KindIncome, domain.CategorySalary, 6000},
		{"outra renda", "BANK", tx("CREDIT", "Transfer - PIX", 100), true, domain.KindIncome, domain.CategoryOther, 100},
		{"aplicação", "BANK", tx("DEBIT", "Fixed income", -500), true, domain.KindTransfer, domain.CategoryInvestment, 500},
		{"resgate", "BANK", tx("CREDIT", "Fixed income", 500), true, domain.KindTransfer, domain.CategoryInvestment, 500},
		{"fatura do cartão", "BANK", tx("DEBIT", "Credit card payment", -900), false, "", "", 0},
		{"entre contas próprias", "BANK", tx("DEBIT", "Same person transfer - PIX", -200), false, "", "", 0},
		{"compra no cartão (valor positivo)", "CREDIT", tx("DEBIT", "Shopping", 120), true, domain.KindExpense, domain.CategoryShopping, 120},
		{"pagamento recebido no cartão", "CREDIT", tx("CREDIT", "", -900), false, "", "", 0},
		{"valor zero", "BANK", tx("DEBIT", "Groceries", 0), false, "", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := toExternal(tc.account, tc.tx)
			if ok != tc.ok {
				t.Fatalf("ok=%v, esperado %v", ok, tc.ok)
			}
			if !ok {
				return
			}
			if got.Kind != tc.kind || got.Category != tc.category || got.Amount != tc.amount {
				t.Errorf("got kind=%s cat=%s amount=%v", got.Kind, got.Category, got.Amount)
			}
		})
	}
}

func TestToExternal_Details(t *testing.T) {
	card := tx("DEBIT", "Shopping", 100)
	card.CreditCardMetadata = &struct {
		InstallmentNumber int `json:"installmentNumber"`
		TotalInstallments int `json:"totalInstallments"`
	}{2, 3}
	got, _ := toExternal("CREDIT", card)
	if got.Description != "Loja (2/3)" || got.PaymentMethod != domain.PaymentMethodCreditCard {
		t.Errorf("parcela/cartão: %q %s", got.Description, got.PaymentMethod)
	}
	if !got.Date.Equal(time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("data: %v", got.Date)
	}

	pix := tx("DEBIT", "Groceries", -5)
	pix.PaymentData = &struct {
		PaymentMethod string `json:"paymentMethod"`
	}{"PIX"}
	if got, _ := toExternal("BANK", pix); got.PaymentMethod != domain.PaymentMethodPix {
		t.Errorf("deveria ser Pix: %s", got.PaymentMethod)
	}

	pending := tx("DEBIT", "Shopping", 1)
	pending.Status = "PENDING"
	if got, _ := toExternal("CREDIT", pending); !got.Pending {
		t.Error("PENDING deveria ser propagado")
	}
}

func TestNextPath(t *testing.T) {
	c := NewClient("id", "secret")
	ok := map[string]string{
		"?accountId=a&after=YWJj%3D%3D":                             "/v2/transactions?accountId=a&after=YWJj%3D%3D",
		"accountId=a&after=YWJj%3D%3D":                              "/v2/transactions?accountId=a&after=YWJj%3D%3D",
		"/v2/transactions?after=x":                                  "/v2/transactions?after=x",
		"https://api.pluggy.ai/v2/transactions?accountId=a&after=x": "/v2/transactions?accountId=a&after=x",
	}
	for in, want := range ok {
		if got, err := c.nextPath(in); err != nil || got != want {
			t.Errorf("nextPath(%q) = %q, %v; esperado %q", in, got, err, want)
		}
	}
	for _, bad := range []string{
		"https://evil.example.com/v2/transactions?after=x", // vazaria a apiKey
		"https://api.pluggy.ai/accounts?itemId=x",
		"/accounts?itemId=x",
	} {
		if _, err := c.nextPath(bad); err == nil {
			t.Errorf("nextPath(%q) deveria ser recusado", bad)
		}
	}
}

// fakePluggy simula a API: /auth, /accounts e /v2/transactions com 2 páginas.
func fakePluggy(t *testing.T, auths *atomic.Int32) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["clientId"] != "id" || body["clientSecret"] != "secret" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		auths.Add(1)
		io.WriteString(w, `{"apiKey":"KEY"}`)
	})
	guard := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-API-KEY") != "KEY" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("/accounts", guard(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("itemId") == "empty" {
			io.WriteString(w, `{"totalPages":1,"results":[]}`)
			return
		}
		io.WriteString(w, `{"totalPages":1,"results":[{"id":"acc1","type":"BANK","name":"  Conta   Corrente ","number":"0001/12345-0","balance":150.5,"taxNumber":"123","owner":"Fulano"}]}`)
	}))
	mux.HandleFunc("/investments", guard(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("itemId") == "empty" {
			io.WriteString(w, `{"total":0,"totalPages":1,"results":[]}`)
			return
		}
		if r.URL.Query().Get("page") == "2" {
			io.WriteString(w, `{"totalPages":2,"results":[{"id":"inv2","type":"MUTUAL_FUND","subtype":"MULTIMARKET_FUND","name":"Fundo X","balance":10.5,"amount":12}]}`)
			return
		}
		io.WriteString(w, `{"totalPages":2,"results":[{"id":"inv1","type":"FIXED_INCOME","subtype":"CDB","name":"  CDB   BANCO  ","balance":1000.25,"amount":1100,"owner":"Fulano","number":"123456","code":"X","issuerCNPJ":"00000000000191"}]}`)
	}))
	mux.HandleFunc("/investments/inv1/transactions", guard(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"totalPages":1,"results":[
			{"id":"m1","type":"BUY","movementType":"CREDIT","amount":1000,"date":"2026-07-10T00:00:00.000Z"},
			{"id":"m2","type":"SELL","movementType":"DEBIT","amount":200.5,"date":"2026-08-15T12:00:00.000Z"},
			{"id":"m3","type":"OTHER","amount":5,"date":"2026-08-16T00:00:00.000Z"}]}`)
	}))
	mux.HandleFunc("/investments/inv2/transactions", guard(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	mux.HandleFunc("/v2/transactions", guard(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("accountId") != "acc1" || q.Get("dateFrom") != "2026-08-01" {
			t.Errorf("query inesperada: %s", r.URL.RawQuery)
		}
		if q.Get("after") == "" {
			io.WriteString(w, `{"results":[{"id":"t1","date":"2026-09-01T00:00:00.000Z","description":"A","amount":-10,"type":"DEBIT","category":"Groceries"}],"next":"?accountId=acc1&dateFrom=2026-08-01&after=P1"}`)
			return
		}
		io.WriteString(w, `{"results":[{"id":"t2","date":"2026-09-02T00:00:00.000Z","description":"B","amount":50,"type":"CREDIT","category":"Salary"}],"next":null}`)
	}))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := NewClient("id", "secret")
	c.baseURL = srv.URL
	return c
}

func TestFetchItem_PaginatesAndAuthenticatesOnce(t *testing.T) {
	var auths atomic.Int32
	c := fakePluggy(t, &auths)

	got, err := c.FetchItem(context.Background(), "item", time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	txs := got.Transactions
	if len(txs) != 2 || txs[0].ID != "t1" || txs[1].ID != "t2" || txs[1].Kind != domain.KindIncome || txs[0].AccountID != "acc1" {
		t.Errorf("transações inesperadas: %+v", txs)
	}
	want := ports.ExternalAccount{ID: "acc1", ItemID: "item", Type: "BANK", Name: "Conta Corrente", Last4: "3450", Balance: 150.5}
	if len(got.Accounts) != 1 || got.Accounts[0] != want {
		t.Errorf("conta inesperada: %+v", got.Accounts)
	}
	if _, err := c.FetchItem(context.Background(), "item", time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if auths.Load() != 1 {
		t.Errorf("a apiKey deveria ser reaproveitada, /auth chamado %d vezes", auths.Load())
	}
}

func TestFetchItem_ReauthenticatesOn401(t *testing.T) {
	var auths atomic.Int32
	c := fakePluggy(t, &auths)
	c.apiKey, c.apiKeyExp = "STALE", time.Now().Add(time.Hour)

	if _, err := c.FetchItem(context.Background(), "item", time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("deveria renovar a apiKey e repetir: %v", err)
	}
	if auths.Load() != 1 {
		t.Errorf("esperava 1 renovação, got %d", auths.Load())
	}
}

func TestFetchItem_ItemWithoutAccounts(t *testing.T) {
	c := fakePluggy(t, new(atomic.Int32))
	if _, err := c.FetchItem(context.Background(), "empty", time.Now()); err != ErrNoAccounts {
		t.Errorf("esperava ErrNoAccounts, got %v", err)
	}
}

func TestAuthFailure_DoesNotLeakSecrets(t *testing.T) {
	c := NewClient("id", "SUPER-SECRET")
	c.baseURL = "http://127.0.0.1:1"
	_, err := c.authenticate(context.Background(), false)
	if err == nil || strings.Contains(err.Error(), "SUPER-SECRET") || strings.Contains(err.Error(), "http://") {
		t.Errorf("erro não deve conter segredo nem a URL da requisição: %v", err)
	}

	bad := fakePluggy(t, new(atomic.Int32))
	bad.clientSecret = "SUPER-SECRET"
	if _, err := bad.authenticate(context.Background(), false); err == nil || strings.Contains(err.Error(), "SUPER-SECRET") {
		t.Errorf("erro de credencial inválida não deve conter o segredo: %v", err)
	}
}

func TestAccountToExternal_CreditCard(t *testing.T) {
	var a account
	err := json.Unmarshal([]byte(`{"id":"c1","type":"CREDIT","name":"Cartão Gold","number":"xxxx8670","balance":1800,
		"owner":"Fulano","taxNumber":"123",
		"creditData":{"creditLimit":5000,"availableCreditLimit":3200,"brand":" mastercard ","balanceCloseDate":"2026-10-03T00:00:00.000Z",
		"balanceDueDate":"2026-10-10","minimumPayment":180.5}}`), &a)
	if err != nil {
		t.Fatal(err)
	}

	got := a.toExternal("item")
	if got.Last4 != "8670" || *got.CreditLimit != 5000 || *got.AvailableCreditLimit != 3200 || got.Brand != "MASTERCARD" || *got.MinimumPayment != 180.5 {
		t.Errorf("cartão inesperado: %+v", got)
	}
	if got.CloseDate.Format("2006-01-02") != "2026-10-03" || got.DueDate.Format("2006-01-02") != "2026-10-10" {
		t.Errorf("datas da fatura: %v %v", got.CloseDate, got.DueDate)
	}
}

func TestAccountToExternal_BankDataAndMissingFields(t *testing.T) {
	var a account
	json.Unmarshal([]byte(`{"id":"b1","type":"BANK","name":"Conta","number":"0001/1234-5","balance":10,"bankData":{"automaticallyInvestedBalance":900}}`), &a) //nolint:errcheck
	got := a.toExternal("item")
	if got.AutoInvested == nil || *got.AutoInvested != 900 || got.CloseDate != nil || got.DueDate != nil || got.Brand != "" {
		t.Errorf("conta inesperada: %+v", got)
	}

	var bad account
	json.Unmarshal([]byte(`{"id":"c","type":"CREDIT","creditData":{"balanceDueDate":"lixo","balanceCloseDate":""}}`), &bad) //nolint:errcheck
	if got := bad.toExternal("i"); got.DueDate != nil || got.CloseDate != nil {
		t.Errorf("data inválida deveria virar nil: %+v", got)
	}
}

func TestInstitution(t *testing.T) {
	respond := map[string]string{
		"ok":     `{"connector":{"name":"  Banco   Itaú ","imageUrl":"https://cdn/logo.svg","primaryColor":"#EC7000"}}`,
		"badc":   `{"connector":{"name":"Banco","primaryColor":"url(javascript:alert(1))"}}`,
		"noname": `{"connector":{"imageUrl":"https://x"}}`,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, `{"apiKey":"k"}`) })
	mux.HandleFunc("/items/", func(w http.ResponseWriter, r *http.Request) {
		body, ok := respond[strings.TrimPrefix(r.URL.Path, "/items/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		io.WriteString(w, body)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := NewClient("id", "secret")
	c.baseURL = srv.URL

	got := c.institution(context.Background(), "ok")
	if got == nil || got.Name != "Banco Itaú" || got.Color != "ec7000" || got.ImageURL != "https://cdn/logo.svg" {
		t.Errorf("instituição inesperada: %+v", got)
	}
	if got := c.institution(context.Background(), "badc"); got == nil || got.Color != "" {
		t.Errorf("cor inválida deve ser descartada: %+v", got)
	}
	if c.institution(context.Background(), "noname") != nil || c.institution(context.Background(), "missing") != nil {
		t.Error("sem nome ou com erro, a instituição deveria ser nil")
	}
}

func TestLast4(t *testing.T) {
	for in, want := range map[string]string{"xxxx8670": "8670", "0001/12345-0": "3450", "12": "12", "": ""} {
		if got := last4(in); got != want {
			t.Errorf("last4(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFetchInvestments_PaginatesAndDropsPersonalData(t *testing.T) {
	c := fakePluggy(t, new(atomic.Int32))
	got, err := c.FetchInvestments(context.Background(), "item")
	if err != nil {
		t.Fatal(err)
	}
	// Movimentações com sinal; tipo desconhecido é descartado. Falha numa posição não derruba as outras.
	m := got[0].Movements
	if len(m) != 2 || m[0].Amount != 1000 || m[1].Amount != -200.5 || m[1].Day.Format("2006-01-02") != "2026-08-15" || got[0].MovementsFailed {
		t.Errorf("movimentações inesperadas: %+v", got[0])
	}
	if !got[1].MovementsFailed || len(got[1].Movements) != 0 || got[1].Balance != 10.5 {
		t.Errorf("falha nas movimentações não pode perder a posição: %+v", got[1])
	}

	want := []ports.ExternalInvestment{
		{ID: "inv1", ItemID: "item", Type: "FIXED_INCOME", Subtype: "CDB", Name: "CDB BANCO", Balance: 1000.25, Amount: 1100},
		{ID: "inv2", ItemID: "item", Type: "MUTUAL_FUND", Subtype: "MULTIMARKET_FUND", Name: "Fundo X", Balance: 10.5, Amount: 12},
	}
	got[0].Movements, got[0].MovementsFailed, got[1].Movements, got[1].MovementsFailed = nil, false, nil, false
	if !reflect.DeepEqual(got, want) {
		t.Errorf("posições inesperadas: %+v", got)
	}

	if none, err := c.FetchInvestments(context.Background(), "empty"); err != nil || len(none) != 0 {
		t.Errorf("item sem investimentos deveria vir vazio: %v %v", none, err)
	}
}

func TestInvestmentName(t *testing.T) {
	long := investment{Name: strings.Repeat("ã", 300)}.toExternal("i")
	if n := len([]rune(long.Name)); n != maxInvestmentNameLen {
		t.Errorf("nome deveria ser truncado em %d runas, got %d", maxInvestmentNameLen, n)
	}
	if got := (investment{Name: "  "}).toExternal("i").Name; got != "Investimento" {
		t.Errorf("nome vazio: %q", got)
	}
}

func described(typ, category, description string, amount float64) transaction {
	t := tx(typ, category, amount)
	t.Description, t.DescriptionRaw = description, strings.ToUpper(description)
	return t
}

func TestToExternal_AutoSweepIsIgnoredButYieldIsKept(t *testing.T) {
	for _, desc := range []string{"Aplicação APL APLIC AUT MAIS", "Resgate RES APLIC AUT MAIS", "Entrada RES APLIC AUT MAIS AP"} {
		typ := "DEBIT"
		if strings.HasPrefix(desc, "Resgate") || strings.HasPrefix(desc, "Entrada") {
			typ = "CREDIT"
		}
		if _, ok := toExternal("BANK", described(typ, "Fixed income", desc, 100)); ok {
			t.Errorf("%q deveria ser ignorada", desc)
		}
	}

	// Aporte feito pelo usuário (Cofrinhos) e rendimento continuam entrando.
	if got, ok := toExternal("BANK", described("DEBIT", "Investment", "Saída APLICACAO COFRINHOS", -50)); !ok || got.Kind != domain.KindTransfer {
		t.Errorf("Cofrinhos deveria virar aplicação: %+v %v", got, ok)
	}
	if got, ok := toExternal("BANK", described("CREDIT", "Income", "Rendimentos REND PAGO APLIC AUT MAIS", 0.12)); !ok || got.Kind != domain.KindIncome {
		t.Errorf("rendimento deveria ser renda: %+v %v", got, ok)
	}
}

func TestToExternal_CategoryFromDescription(t *testing.T) {
	cases := []struct {
		category, description string
		want                  domain.Category
	}{
		{"", "Pagamento de Pix QR Code 99 FOOD", domain.CategoryFood},
		{"", "Compra débito Auto Posto Central Ltda", domain.CategoryTransport},
		{"", "Pagamento de Pix QR Code RD SAUDE", domain.CategoryHealth},
		{"", "Pagamento de boleto HOSPITAL SANTA CASA DE EXEMPLO", domain.CategoryHealth},
		{"", "ANTHROPIC* CLAUDE SUBSAN FRANCISCOUSA", domain.CategoryEntertainment},
		{"", "Compra débito Aracatuba*kalunga 0148", domain.CategoryShopping},
		{"", "Pix enviado Maria da Silva", domain.CategoryOther},
		{"", "Pagamento de Pix QR Code TELEFONICA BRAS", domain.CategoryOther},
		{"", "Compra débito Composto Ltda", domain.CategoryOther}, // "posto" só vale como palavra
		// A categoria do Pluggy, quando útil, manda: a descrição só desempata o OTHER.
		{"Groceries", "Pagamento de Pix QR Code 99 FOOD", domain.CategoryMarket},
	}
	for _, tc := range cases {
		got, ok := toExternal("BANK", described("DEBIT", tc.category, tc.description, -20))
		if !ok || got.Category != tc.want {
			t.Errorf("%q (%q): got %s, esperado %s", tc.description, tc.category, got.Category, tc.want)
		}
	}

	// Renda nunca é reclassificada por descrição.
	if got, _ := toExternal("BANK", described("CREDIT", "", "Pix recebido IFOOD", 10)); got.Category != domain.CategoryOther {
		t.Errorf("renda deveria continuar OTHER: %s", got.Category)
	}
}

func TestMovementSign(t *testing.T) {
	cases := []struct {
		m    movement
		want float64
		ok   bool
	}{
		{movement{ID: "a", Type: "BUY", MovementType: "CREDIT", Amount: 10, Date: "2026-01-02T00:00:00Z"}, 10, true},
		{movement{ID: "a", Type: "SELL", MovementType: "DEBIT", Amount: 10, Date: "2026-01-02T00:00:00Z"}, -10, true},
		{movement{ID: "a", Type: "TAX", MovementType: "DEBIT", Amount: 3, Date: "2026-01-02T00:00:00Z"}, -3, true},
		{movement{ID: "a", Type: "BUY", Amount: 10, Date: "2026-01-02T00:00:00Z"}, 10, true},   // sem movementType
		{movement{ID: "a", Type: "SELL", Amount: 10, Date: "2026-01-02T00:00:00Z"}, -10, true}, // idem
		{movement{ID: "a", Type: "TRANSFER", Amount: 10, Date: "2026-01-02T00:00:00Z"}, 0, false},
		{movement{ID: "a", Type: "BUY", MovementType: "CREDIT", Amount: -10, Date: "2026-01-02T00:00:00Z"}, 10, true}, // amount sem sinal
		{movement{ID: "", Type: "BUY", MovementType: "CREDIT", Amount: 10, Date: "2026-01-02T00:00:00Z"}, 0, false},
		{movement{ID: "a", Type: "BUY", MovementType: "CREDIT", Amount: 0, Date: "2026-01-02T00:00:00Z"}, 0, false},
		{movement{ID: "a", Type: "BUY", MovementType: "CREDIT", Amount: 10, Date: "ontem"}, 0, false},
		{movement{ID: "a", Type: "BUY", MovementType: "CREDIT", Amount: 10, TradeDate: "2026-03-04T00:00:00Z"}, 10, true}, // cai na data de negociação
	}
	for i, tc := range cases {
		got, ok := tc.m.toExternal()
		if ok != tc.ok || (ok && got.Amount != tc.want) {
			t.Errorf("caso %d: got %+v ok=%v, esperado %v ok=%v", i, got, ok, tc.want, tc.ok)
		}
	}
}

func TestToExternal_SalaryAndInvestmentCategories(t *testing.T) {
	salary, ok := toExternal("BANK", described("CREDIT", "Transfer - PIX", "Salário EMPRESA EXEMPLO LTDA", 5000))
	if !ok || salary.Kind != domain.KindIncome || salary.Category != domain.CategorySalary {
		t.Errorf("salário pela descrição: %+v", salary)
	}
	other, _ := toExternal("BANK", described("CREDIT", "Transfer - PIX", "Pix recebido FULANO", 50))
	if other.Category != domain.CategoryOther {
		t.Errorf("outra renda continua OTHER: %s", other.Category)
	}
	inv, _ := toExternal("BANK", described("DEBIT", "Investment", "Saída APLICACAO COFRINHOS", -100))
	if inv.Kind != domain.KindTransfer || inv.Category != domain.CategoryInvestment {
		t.Errorf("aplicação é INVESTMENT: %+v", inv)
	}
}
