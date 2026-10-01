package pluggy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
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
		io.WriteString(w, `{"totalPages":1,"results":[{"id":"acc1","type":"BANK"}]}`)
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

func TestFetchTransactions_PaginatesAndAuthenticatesOnce(t *testing.T) {
	var auths atomic.Int32
	c := fakePluggy(t, &auths)

	got, err := c.FetchTransactions(context.Background(), "item", time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "t1" || got[1].ID != "t2" || got[1].Kind != domain.KindIncome {
		t.Errorf("transações inesperadas: %+v", got)
	}
	if _, err := c.FetchTransactions(context.Background(), "item", time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if auths.Load() != 1 {
		t.Errorf("a apiKey deveria ser reaproveitada, /auth chamado %d vezes", auths.Load())
	}
}

func TestFetchTransactions_ReauthenticatesOn401(t *testing.T) {
	var auths atomic.Int32
	c := fakePluggy(t, &auths)
	c.apiKey, c.apiKeyExp = "STALE", time.Now().Add(time.Hour)

	if _, err := c.FetchTransactions(context.Background(), "item", time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("deveria renovar a apiKey e repetir: %v", err)
	}
	if auths.Load() != 1 {
		t.Errorf("esperava 1 renovação, got %d", auths.Load())
	}
}

func TestFetchTransactions_ItemWithoutAccounts(t *testing.T) {
	c := fakePluggy(t, new(atomic.Int32))
	if _, err := c.FetchTransactions(context.Background(), "empty", time.Now()); err != ErrNoAccounts {
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
