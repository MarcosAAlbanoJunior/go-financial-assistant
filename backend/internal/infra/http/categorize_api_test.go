package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/infra/gemini"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

func categorizeReader() *fakeReader {
	last := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	return &fakeReader{uncategorized: []domain.UncategorizedGroup{
		{Key: "pix enviado fulano", Label: "Pix enviado Fulano de Tal", Count: 10, Total: 5000, Last: last},
		{Key: "padaria do ze", Label: "PADARIA DO ZE 123456", Count: 8, Total: 400, Last: last},
		{Key: "condominio", Label: "Condominio Edificio", Count: 3, Total: 900, Last: last},
	}}
}

type categorizeBody struct {
	Categories []struct{ Value, Label string } `json:"categories"`
	Groups     []struct {
		Key      string `json:"key"`
		Transfer bool   `json:"transfer"`
	} `json:"groups"`
	AI struct {
		Enabled       bool            `json:"enabled"`
		BlockedReason string          `json:"blockedReason"`
		Hash          string          `json:"hash"`
		Context       json.RawMessage `json:"context"`
	} `json:"ai"`
}

func getCategorize(t *testing.T, s *Server, c *http.Cookie) categorizeBody {
	t.Helper()
	rec := do(s, "GET", "/api/categorize", "", nil, c)
	var b categorizeBody
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &b) != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	return b
}

func TestAPI_Categorize(t *testing.T) {
	s := newCoachAPI(t, categorizeReader(), &fakeCoach{}, true)
	c := login(t, s)
	b := getCategorize(t, s, c)

	if len(b.Groups) != 3 || !b.Groups[0].Transfer || b.Groups[1].Transfer {
		t.Errorf("lista com os Pix marcados: %+v", b.Groups)
	}
	ctx := string(b.AI.Context)
	if !b.AI.Enabled || !strings.Contains(ctx, `"nome":"PADARIA DO ZE"`) || !strings.Contains(ctx, `"nome":"Condominio Edificio"`) {
		t.Errorf("a prévia da IA tem o comércio: %s", ctx)
	}
	if strings.Contains(strings.ToLower(ctx), "pix") || strings.Contains(ctx, "Fulano") || strings.Contains(ctx, "123456") {
		t.Errorf("Pix e números nunca vão à IA: %s", ctx)
	}
	hasPeople, hasOther := false, false
	for _, o := range b.Categories {
		hasPeople = hasPeople || o.Value == "PEOPLE"
		hasOther = hasOther || (o.Value == "OTHER" && o.Label == "Outros")
	}
	if !hasPeople || !hasOther {
		t.Errorf("categorias atribuíveis: %+v", b.Categories)
	}
	if rec := do(s, "GET", "/api/categorize", "", nil); rec.Code != 401 {
		t.Errorf("sem sessão = %d", rec.Code)
	}
}

func TestAPI_CategorizeBlockedWithoutPaidPlan(t *testing.T) {
	s := newCoachAPI(t, categorizeReader(), &fakeCoach{}, false)
	c := login(t, s)
	b := getCategorize(t, s, c)
	if b.AI.Enabled || !strings.Contains(b.AI.BlockedReason, "GEMINI_PAID_PLAN") {
		t.Errorf("bloqueado: %+v", b.AI)
	}
	coach := &fakeCoach{}
	s = newCoachAPI(t, categorizeReader(), coach, false)
	c = login(t, s)
	if rec := do(s, "POST", "/api/categorize/suggest", `{"hash":"`+getCategorize(t, s, c).AI.Hash+`"}`, jsonHeader, c); rec.Code != 403 || coach.calls != 0 {
		t.Errorf("sem plano pago nada é enviado: %d calls=%d", rec.Code, coach.calls)
	}
}

func TestAPI_CategorizeSuggest(t *testing.T) {
	coach := &fakeCoach{cats: []ports.CategorySuggestion{
		{ID: "c1", Category: "FOOD"},    // padaria
		{ID: "c2", Category: "HOUSING"}, // condomínio
		{ID: "c3", Category: "FOOD"},    // não existe
		{ID: "c1", Category: "PEOPLE"},  // repetida e não permitida
	}}
	s := newCoachAPI(t, categorizeReader(), coach, true)
	c := login(t, s)
	h := getCategorize(t, s, c).AI

	rec := do(s, "POST", "/api/categorize/suggest", `{"hash":"`+h.Hash+`"}`, jsonHeader, c)
	body := rec.Body.String()
	if rec.Code != 200 || coach.calls != 1 || string(coach.payload) != string(h.Context) {
		t.Fatalf("uma chamada com exatamente a prévia: %d calls=%d %s", rec.Code, coach.calls, body)
	}
	if !strings.Contains(body, `{"key":"padaria do ze","category":"FOOD"}`) || !strings.Contains(body, `{"key":"condominio","category":"HOUSING"}`) || strings.Contains(body, "PEOPLE") || strings.Count(body, `"key"`) != 2 {
		t.Errorf("só as sugestões válidas, ligadas às contas: %s", body)
	}
}

func TestAPI_CategorizeSuggestRefusals(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		hdr  map[string]string
		want int
	}{
		"dados mudaram":    {`{"hash":"abc"}`, jsonHeader, 409},
		"corpo inválido":   {`nao-json`, jsonHeader, 400},
		"formulário":       {`hash=x`, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, 403},
		"origem diferente": {`{"hash":"x"}`, map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, 403},
	} {
		t.Run(name, func(t *testing.T) {
			coach := &fakeCoach{}
			s := newCoachAPI(t, categorizeReader(), coach, true)
			c := login(t, s)
			if rec := do(s, "POST", "/api/categorize/suggest", tc.body, tc.hdr, c); rec.Code != tc.want || coach.calls != 0 {
				t.Errorf("%d (esperava %d), chamadas à IA: %d", rec.Code, tc.want, coach.calls)
			}
		})
	}
	s := newCoachAPI(t, categorizeReader(), &fakeCoach{err: http.ErrAbortHandler}, true)
	c := login(t, s)
	rec := do(s, "POST", "/api/categorize/suggest", `{"hash":"`+getCategorize(t, s, c).AI.Hash+`"}`, jsonHeader, c)
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "abort") {
		t.Errorf("erro do provedor não vaza: %d %s", rec.Code, rec.Body)
	}
}

func TestAPI_SetCategoryRule(t *testing.T) {
	r := categorizeReader()
	s := newCoachAPI(t, r, &fakeCoach{}, true)
	c := login(t, s)
	put := func(body string) int { return do(s, "PUT", "/api/categorize/rules", body, jsonHeader, c).Code }

	if rec := do(s, "PUT", "/api/categorize/rules", `{"key":"condominio","category":"HOUSING"}`, jsonHeader, c); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"changed":3`) || len(r.rules2) != 1 || r.rules2[0] != (domain.CategoryRule{Key: "condominio", Category: "HOUSING"}) {
		t.Errorf("gravar: %d %s %+v", rec.Code, rec.Body, r.rules2)
	}
	if put(`{"key":"padaria do ze","category":"OTHER"}`) != 200 {
		t.Error("manter em Outros é válido")
	}
	for name, body := range map[string]string{
		"categoria de renda":  `{"key":"x","category":"SALARY"}`,
		"categoria inventada": `{"key":"x","category":"BANANA"}`,
		"chave com SQL":       `{"key":"x'; drop table purchases;--","category":"FOOD"}`,
		"chave maiúscula":     `{"key":"Condominio","category":"FOOD"}`,
		"chave vazia":         `{"key":"","category":"FOOD"}`,
		"corpo inválido":      `nao-json`,
	} {
		if code := put(body); code != 400 {
			t.Errorf("%s: %d, esperava 400", name, code)
		}
	}
	if len(r.rules2) != 2 {
		t.Errorf("nada inválido pode ser gravado: %+v", r.rules2)
	}
	if code := do(s, "PUT", "/api/categorize/rules", `key=x`, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, c).Code; code != 403 {
		t.Errorf("form (CSRF) = %d", code)
	}
	if code := do(s, "PUT", "/api/categorize/rules", `{"key":"x","category":"FOOD"}`, map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, c).Code; code != 403 {
		t.Errorf("origem diferente = %d", code)
	}
	if code := do(s, "PUT", "/api/categorize/rules", `{"key":"x","category":"FOOD"}`, jsonHeader).Code; code != 401 {
		t.Errorf("sem sessão = %d", code)
	}
}

// As categorias do esquema enviado ao Gemini e as que o servidor aceita validar são as mesmas.
func TestCategorizeEnumMatchesUsecase(t *testing.T) {
	var want []string
	for _, c := range usecase.AICategories {
		want = append(want, string(c))
	}
	if strings.Join(gemini.CategorizeEnum, ",") != strings.Join(want, ",") {
		t.Errorf("gemini %v != usecase %v", gemini.CategorizeEnum, want)
	}
}
