package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/openfinance"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

var jsonHdr = map[string]string{"Content-Type": "application/json"}

func balancesFixture() (*fakeReader, uuid.UUID) {
	inst := uuid.New()
	limit, avail := 1000.0, 220.0
	due := time.Now().AddDate(0, 0, 2)
	return &fakeReader{
		institutions: []domain.Institution{{ID: inst, ItemID: "SEGREDO-ITEM-ID", Name: "Itaú", Color: "ec7000", HasLogo: true}},
		accounts: []domain.Account{
			{ID: uuid.New(), ItemID: "SEGREDO-ITEM-ID", InstitutionID: &inst, Type: "BANK", Name: "Itaú", Last4: "5678", Balance: 4345.67, UpdatedAt: time.Now()},
			{ID: uuid.New(), ItemID: "SEGREDO-ITEM-ID", InstitutionID: &inst, Type: "CREDIT", Name: "Click", Last4: "3456", Balance: 940,
				CreditLimit: &limit, AvailableCreditLimit: &avail, DueDate: &due, Brand: "MASTERCARD", UpdatedAt: time.Now()},
		},
		logos: map[uuid.UUID]fakeLogo{inst: {data: []byte("<svg xmlns='http://www.w3.org/2000/svg'/>"), mime: "image/svg+xml"}},
	}, inst
}

func TestAPI_Balances(t *testing.T) {
	r, inst := balancesFixture()
	s := newTestAPI(t, r)
	c := login(t, s)

	rec := do(s, "GET", "/api/balances", "", nil, c)
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	if strings.Contains(rec.Body.String(), "SEGREDO-ITEM-ID") {
		t.Fatal("o item_id do Pluggy não pode sair na API")
	}
	var got struct {
		TotalInAccount *float64 `json:"totalInAccount"`
		OpenInvoices   float64  `json:"openInvoices"`
		Institutions   []struct {
			ID    string  `json:"id"`
			Name  string  `json:"name"`
			Color string  `json:"color"`
			Logo  string  `json:"logo"`
			Share float64 `json:"shareOfTotal"`
			Cards []struct {
				Invoice    float64 `json:"invoice"`
				UsageLevel string  `json:"usageLevel"`
				DueLevel   string  `json:"dueLevel"`
				DaysToDue  *int    `json:"daysToDue"`
				Brand      string  `json:"brand"`
			} `json:"cards"`
		} `json:"institutions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.TotalInAccount == nil || *got.TotalInAccount != 4345.67 || got.OpenInvoices != 940 || len(got.Institutions) != 1 {
		t.Fatalf("resposta inesperada: %s", rec.Body)
	}
	in := got.Institutions[0]
	if in.ID != inst.String() || in.Color != "#ec7000" || in.Logo != "/api/institutions/"+inst.String()+"/logo" || in.Share != 1 {
		t.Errorf("instituição: %+v", in)
	}
	card := in.Cards[0]
	if card.UsageLevel != "warning" || card.DueLevel != "warning" || card.DaysToDue == nil || card.Brand != "MASTERCARD" {
		t.Errorf("cartão: %+v", card)
	}
}

func TestAPI_BalancesEmptyAndErrors(t *testing.T) {
	s := newTestAPI(t, &fakeReader{})
	rec := do(s, "GET", "/api/balances", "", nil, login(t, s))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"institutions":[]`) || !strings.Contains(rec.Body.String(), `"totalInAccount":null`) {
		t.Errorf("sem contas: %d %s", rec.Code, rec.Body)
	}
	s = newTestAPI(t, &fakeReader{err: errors.New("pq: segredo do banco")})
	rec = do(s, "GET", "/api/balances", "", nil, login(t, s))
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "segredo") {
		t.Errorf("erro deveria ser genérico: %d %s", rec.Code, rec.Body)
	}
}

func TestAPI_BalancesRoutesRequireSession(t *testing.T) {
	r, inst := balancesFixture()
	s := newTestAPI(t, r)
	for _, p := range [][2]string{{"GET", "/api/balances"}, {"GET", "/api/institutions/" + inst.String() + "/logo"}, {"POST", "/api/sync"}} {
		if rec := do(s, p[0], p[1], "", jsonHdr); rec.Code != 401 {
			t.Errorf("%s %s sem sessão = %d", p[0], p[1], rec.Code)
		}
	}
}

func TestAPI_InstitutionLogo(t *testing.T) {
	r, inst := balancesFixture()
	s := newTestAPI(t, r)
	c := login(t, s)

	rec := do(s, "GET", "/api/institutions/"+inst.String()+"/logo", "", nil, c)
	h := rec.Header()
	if rec.Code != 200 || h.Get("Content-Type") != "image/svg+xml" || h.Get("X-Content-Type-Options") != "nosniff" ||
		h.Get("Content-Security-Policy") != "default-src 'none'; sandbox" || !strings.Contains(rec.Body.String(), "<svg") {
		t.Errorf("logo: %d %v", rec.Code, h)
	}
	for _, id := range []string{uuid.NewString(), "nao-e-uuid", "%27%20OR%201=1"} {
		if rec := do(s, "GET", "/api/institutions/"+id+"/logo", "", nil, c); rec.Code != 404 {
			t.Errorf("%q = %d, esperava 404", id, rec.Code)
		}
	}
}

type fakeSyncer struct {
	res   openfinance.SyncResult
	err   error
	calls int
}

func (f *fakeSyncer) Sync(context.Context) (openfinance.SyncResult, error) {
	f.calls++
	return f.res, f.err
}

func newSyncAPI(t *testing.T, syncer *fakeSyncer) *Server {
	t.Helper()
	s := NewServer(0, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if syncer != nil {
		s.SetSyncer(syncer)
	}
	if err := s.MountAPI(StaticPassword(testPassword), &fakeReader{}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAPI_SyncNow(t *testing.T) {
	f := &fakeSyncer{res: openfinance.SyncResult{Inserted: 2, Existing: 5}}
	s := newSyncAPI(t, f)
	c := login(t, s)

	rec := do(s, "POST", "/api/sync", "{}", jsonHdr, c)
	if rec.Code != 200 || f.calls != 1 || !strings.Contains(rec.Body.String(), `"inserted":2`) || !strings.Contains(rec.Body.String(), `"partial":false`) {
		t.Fatalf("%d %s calls=%d", rec.Code, rec.Body, f.calls)
	}

	// CSRF: formulário de outro site ou Origin diferente.
	if rec := do(s, "POST", "/api/sync", "", map[string]string{"Content-Type": "text/plain"}, c); rec.Code != 403 {
		t.Errorf("sem JSON = %d", rec.Code)
	}
	if rec := do(s, "POST", "/api/sync", "{}", map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, c); rec.Code != 403 {
		t.Errorf("cross-origin = %d", rec.Code)
	}
	if f.calls != 1 {
		t.Errorf("requisições recusadas não podem sincronizar: %d", f.calls)
	}
}

func TestAPI_SyncNowStates(t *testing.T) {
	s := newSyncAPI(t, &fakeSyncer{err: openfinance.ErrSyncInProgress})
	if rec := do(s, "POST", "/api/sync", "{}", jsonHdr, login(t, s)); rec.Code != 409 {
		t.Errorf("em andamento = %d", rec.Code)
	}

	s = newSyncAPI(t, &fakeSyncer{res: openfinance.SyncResult{Inserted: 1}, err: errors.New("item x: pluggy falhou com detalhe interno")})
	rec := do(s, "POST", "/api/sync", "{}", jsonHdr, login(t, s))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"partial":true`) || strings.Contains(rec.Body.String(), "detalhe interno") {
		t.Errorf("falha parcial: %d %s", rec.Code, rec.Body)
	}

	s = newSyncAPI(t, nil)
	if rec := do(s, "POST", "/api/sync", "{}", jsonHdr, login(t, s)); rec.Code != 503 {
		t.Errorf("sem Open Finance = %d", rec.Code)
	}
}

func TestAPI_SyncNowRateLimited(t *testing.T) {
	s := newSyncAPI(t, &fakeSyncer{})
	c := login(t, s)
	var last int
	for i := 0; i < 5; i++ {
		last = do(s, "POST", "/api/sync", "{}", jsonHdr, c).Code
	}
	if last != 429 {
		t.Errorf("deveria limitar a taxa, último = %d", last)
	}
}

func TestAPI_SyncNowReportsDataAge(t *testing.T) {
	at := time.Date(2026, 10, 2, 17, 42, 0, 0, time.UTC)
	older := at.Add(-5 * time.Hour)
	s := NewServer(0, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.SetSyncer(&fakeSyncer{})
	reader := &fakeReader{institutions: []domain.Institution{{ID: uuid.New(), Name: "A", SourceUpdatedAt: &at}, {ID: uuid.New(), Name: "B", SourceUpdatedAt: &older}, {ID: uuid.New(), Name: "C"}}}
	if err := s.MountAPI(StaticPassword(testPassword), reader); err != nil {
		t.Fatal(err)
	}
	rec := do(s, "POST", "/api/sync", "{}", jsonHdr, login(t, s))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"dataAsOf":"2026-10-02T12:42:00Z"`) {
		t.Errorf("deveria devolver a atualização mais antiga do Pluggy: %d %s", rec.Code, rec.Body)
	}
}
