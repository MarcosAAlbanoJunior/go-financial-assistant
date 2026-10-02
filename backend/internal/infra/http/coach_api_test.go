package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

type fakeCoach struct {
	advice  ports.CoachAdvice
	err     error
	calls   int
	payload []byte
}

func (f *fakeCoach) Advise(_ context.Context, payload []byte) (ports.CoachAdvice, error) {
	f.calls++
	f.payload = payload
	return f.advice, f.err
}

// coachReader tem uma conta fixa (netflix, pelo fake), uma duplicata de Pix e uma meta.
func coachReader() *fakeReader {
	d := func(day int) time.Time { return time.Date(2026, 11, day, 0, 0, 0, 0, time.UTC) }
	return &fakeReader{
		payments: []ports.ExpensePayment{
			{Key: "pix enviado fulano de tal", Label: "Pix enviado Fulano de Tal", Category: "OTHER", PaymentMethod: "PIX", Date: d(2), Amount: 300},
			{Key: "pix enviado fulano de tal", Label: "Pix enviado Fulano de Tal", Category: "OTHER", PaymentMethod: "PIX", Date: d(3), Amount: 300},
		},
		goals: []ports.Goal{{Kind: ports.GoalReserve, Name: "Reserva", ReserveMonths: 6, CreatedAt: time.Now()}},
	}
}

func newCoachAPI(t *testing.T, r *fakeReader, coach ports.Coach, paid bool) *Server {
	t.Helper()
	s := newTestAPI(t, r)
	s.SetCoach(coach, paid)
	s.mux = http.NewServeMux()
	if err := s.mountAPI(testPassword, r, func() time.Time { return time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC) }); err != nil {
		t.Fatal(err)
	}
	return s
}

var jsonHeader = map[string]string{"Content-Type": "application/json"}

type previewBody struct {
	Enabled       bool            `json:"enabled"`
	BlockedReason string          `json:"blockedReason"`
	Hash          string          `json:"hash"`
	Bytes         int             `json:"bytes"`
	Context       json.RawMessage `json:"context"`
}

func preview(t *testing.T, s *Server, c *http.Cookie) previewBody {
	t.Helper()
	rec := do(s, "GET", "/api/coach/preview?month=2026-11", "", nil, c)
	if rec.Code != 200 {
		t.Fatalf("prévia = %d %s", rec.Code, rec.Body)
	}
	var p previewBody
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAPI_CoachPreview(t *testing.T) {
	for name, tc := range map[string]struct {
		coach ports.Coach
		paid  bool
		want  bool
	}{
		"plano pago":          {&fakeCoach{}, true, true},
		"plano não declarado": {&fakeCoach{}, false, false},
		"sem cliente":         {nil, true, false},
	} {
		t.Run(name, func(t *testing.T) {
			s := newCoachAPI(t, coachReader(), tc.coach, tc.paid)
			p := preview(t, s, login(t, s))
			if p.Enabled != tc.want || (!tc.want && !strings.Contains(p.BlockedReason, "GEMINI_PAID_PLAN")) {
				t.Errorf("enabled=%v motivo=%q", p.Enabled, p.BlockedReason)
			}
			ctx := string(p.Context)
			// O contexto mostrado é o enviado: sem o nome da pessoa do Pix, com os números calculados pelo código.
			for _, want := range []string{`"mes":"2026-11"`, "transferência para pessoa", `"id":"s1"`, `"id":"m1"`, `"economia_possivel"`} {
				if !strings.Contains(ctx, want) {
					t.Errorf("falta %s em %s", want, ctx)
				}
			}
			if strings.Contains(ctx, "Fulano") || strings.Contains(strings.ToLower(ctx), "pix") || p.Hash == "" || p.Bytes != len(p.Context) {
				t.Errorf("vazamento ou prévia inconsistente: %s", ctx)
			}
		})
	}
	s := newCoachAPI(t, coachReader(), &fakeCoach{}, true)
	if rec := do(s, "GET", "/api/coach/preview?month=2026-11", "", nil); rec.Code != 401 {
		t.Errorf("sem sessão = %d", rec.Code)
	}
	if rec := do(s, "GET", "/api/coach/preview", "", nil, login(t, s)); rec.Code != 400 {
		t.Errorf("sem mês = %d", rec.Code)
	}
}

func TestAPI_CoachAnalyze(t *testing.T) {
	coach := &fakeCoach{advice: ports.CoachAdvice{
		Summary: "As contas fixas pesam.",
		Actions: []ports.CoachAction{
			{SuggestionID: "s1", Priority: 1, Comment: "Vale rever esta conta.", Question: "Você ainda usa?"},
			{SuggestionID: "s9", Priority: 1, Comment: "Inventada."},
			{SuggestionID: "s2", Priority: 2, Comment: "Economiza R$ 999 por mês."},
		},
		Goals:     []ports.CoachNote{{GoalID: "m1", Comment: "Está no caminho."}},
		Questions: []string{"Mudou algo na rotina?"},
	}}
	s := newCoachAPI(t, coachReader(), coach, true)
	c := login(t, s)
	p := preview(t, s, c)

	rec := do(s, "POST", "/api/coach/analyze", `{"month":"2026-11","hash":"`+p.Hash+`"}`, jsonHeader, c)
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, body)
	}
	if coach.calls != 1 || string(coach.payload) != string(p.Context) {
		t.Errorf("uma chamada com exatamente o contexto da prévia: calls=%d", coach.calls)
	}
	for _, want := range []string{
		`"summary":"As contas fixas pesam."`, `"suggestionId":"s1"`, `"comment":"Vale rever esta conta."`, `"question":"Você ainda usa?"`,
		`"kind":"`, `"annual":`, `"name":"Reserva"`, `"questions":["Mudou algo na rotina?"]`, `"callsLeft":9`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("falta %s em %s", want, body)
		}
	}
	if strings.Contains(body, "s9") || strings.Contains(body, "999") {
		t.Errorf("ID desconhecido e número inventado pela IA devem sumir: %s", body)
	}
}

func TestAPI_CoachAnalyze_Refusals(t *testing.T) {
	hash := func(s *Server, c *http.Cookie) string { return preview(t, s, c).Hash }
	for name, tc := range map[string]struct {
		paid    bool
		body    func(h string) string
		hdr     map[string]string
		session bool
		want    int
	}{
		"plano não declarado": {false, func(h string) string { return `{"month":"2026-11","hash":"` + h + `"}` }, jsonHeader, true, 403},
		"dados mudaram":       {true, func(string) string { return `{"month":"2026-11","hash":"abc"}` }, jsonHeader, true, 409},
		"sem hash da prévia":  {true, func(string) string { return `{"month":"2026-11"}` }, jsonHeader, true, 409},
		"mês inválido":        {true, func(h string) string { return `{"month":"x","hash":"` + h + `"}` }, jsonHeader, true, 400},
		"corpo inválido":      {true, func(string) string { return `nao-json` }, jsonHeader, true, 400},
		"formulário (CSRF)":   {true, func(string) string { return `month=2026-11` }, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, true, 403},
		"origem diferente":    {true, func(h string) string { return `{"month":"2026-11","hash":"` + h + `"}` }, map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, true, 403},
		"sem sessão":          {true, func(h string) string { return `{"month":"2026-11","hash":"` + h + `"}` }, jsonHeader, false, 401},
	} {
		t.Run(name, func(t *testing.T) {
			coach := &fakeCoach{}
			s := newCoachAPI(t, coachReader(), coach, tc.paid)
			c := login(t, s)
			var cookies []*http.Cookie
			if tc.session {
				cookies = append(cookies, c)
			}
			if rec := do(s, "POST", "/api/coach/analyze", tc.body(hash(s, c)), tc.hdr, cookies...); rec.Code != tc.want {
				t.Errorf("%d, esperava %d: %s", rec.Code, tc.want, rec.Body)
			}
			if coach.calls != 0 {
				t.Errorf("nada pode ser enviado à IA: %d chamadas", coach.calls)
			}
		})
	}
}

func TestAPI_CoachAnalyze_AIFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		coach *fakeCoach
		want  int
	}{
		"erro do Gemini":       {&fakeCoach{err: errors.New("segredo: falha interna do provedor")}, 500},
		"resposta sem validez": {&fakeCoach{advice: ports.CoachAdvice{Summary: "Gastou R$ 500."}}, 502},
	} {
		t.Run(name, func(t *testing.T) {
			s := newCoachAPI(t, coachReader(), tc.coach, true)
			c := login(t, s)
			rec := do(s, "POST", "/api/coach/analyze", `{"month":"2026-11","hash":"`+preview(t, s, c).Hash+`"}`, jsonHeader, c)
			if rec.Code != tc.want || strings.Contains(rec.Body.String(), "segredo") {
				t.Errorf("%d: %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestCoachService_LimitsAndConcurrency(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	c := newCoachService(&fakeCoach{}, true)

	if ok, _ := c.begin(now); !ok {
		t.Fatal("primeira análise deveria passar")
	}
	if ok, reason := c.begin(now); ok || !strings.Contains(reason, "andamento") {
		t.Errorf("duas ao mesmo tempo: %v %q", ok, reason)
	}
	c.end()
	for i := 1; i < coachDailyLimit; i++ {
		if ok, _ := c.begin(now); !ok {
			t.Fatalf("análise %d deveria passar", i+1)
		}
		c.end()
	}
	if ok, reason := c.begin(now); ok || !strings.Contains(reason, "limite") || c.callsLeft(now) != 0 {
		t.Errorf("teto diário: %v %q %d", ok, reason, c.callsLeft(now))
	}
	if ok, _ := c.begin(now.AddDate(0, 0, 1)); !ok {
		t.Error("o teto zera no dia seguinte")
	}
	if usecase.MaxCoachBytes <= 0 {
		t.Fatal("limite de tamanho precisa existir")
	}
}
