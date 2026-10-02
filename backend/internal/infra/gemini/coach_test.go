package gemini

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/genai"
)

func TestParseCoachResponse(t *testing.T) {
	raw := `{"resumo":"Visão geral.","acoes":[{"sugestao_id":"s1","prioridade":2,"comentario":"Rever.","pergunta":"Ainda usa?"}],"metas":[{"meta_id":"m1","comentario":"No caminho."}],"perguntas":["Mudou algo?"]}`
	got, err := parseCoachResponse(makeResponse(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "Visão geral." || len(got.Actions) != 1 || got.Actions[0].SuggestionID != "s1" || got.Actions[0].Priority != 2 || got.Actions[0].Question != "Ainda usa?" {
		t.Errorf("ações: %+v", got)
	}
	if len(got.Goals) != 1 || got.Goals[0].GoalID != "m1" || len(got.Questions) != 1 {
		t.Errorf("metas e perguntas: %+v", got)
	}
}

func TestParseCoachResponse_Invalid(t *testing.T) {
	for name, resp := range map[string]*genai.GenerateContentResponse{
		"nil":      nil,
		"vazia":    {},
		"não JSON": makeResponse("isto não é json"),
		"truncada": makeResponse(`{"resumo":"corta`),
	} {
		if _, err := parseCoachResponse(resp); err == nil {
			t.Errorf("%s: esperava erro", name)
		}
	}
}

// Advise manda uma única chamada ao modelo escolhido, com o contexto como dado e o esquema JSON na configuração.
func TestAdvise_RequestShape(t *testing.T) {
	var path string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"{\"resumo\":\"ok\",\"acoes\":[]}"}]}}]}`))
	}))
	defer srv.Close()

	gc, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey: "chave-de-teste", Backend: genai.BackendGeminiAPI, HTTPOptions: genai.HTTPOptions{BaseURL: srv.URL},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{client: gc, coachModel: "modelo-de-teste"}

	got, err := c.Advise(context.Background(), []byte(`{"mes":"2026-09"}`))
	if err != nil || got.Summary != "ok" {
		t.Fatalf("resposta: %+v %v", got, err)
	}
	if !strings.Contains(path, "modelo-de-teste") {
		t.Errorf("modelo configurado não foi usado: %s", path)
	}
	text, _ := json.Marshal(body)
	for _, want := range []string{`"responseMimeType":"application/json"`, `"responseSchema"`, `"maxOutputTokens":2048`, `{\"mes\":\"2026-09\"}`, "NÃO escreva nenhum número"} {
		if !strings.Contains(string(text), strings.ReplaceAll(want, `\"`, `\"`)) {
			t.Errorf("falta %s na requisição: %s", want, text)
		}
	}
}

func TestParseCategorizeResponse(t *testing.T) {
	got, err := parseCategorizeResponse(makeResponse(`{"sugestoes":[{"id":"c1","categoria":"FOOD"},{"id":"c2","categoria":"BILLS"}]}`))
	if err != nil || len(got) != 2 || got[0].ID != "c1" || got[1].Category != "BILLS" {
		t.Fatalf("%+v %v", got, err)
	}
	for name, resp := range map[string]*genai.GenerateContentResponse{"nil": nil, "vazia": {}, "não JSON": makeResponse("ops")} {
		if _, err := parseCategorizeResponse(resp); err == nil {
			t.Errorf("%s: esperava erro", name)
		}
	}
}

// O esquema só deixa a IA responder com as categorias que o app aceita sugerir.
func TestCategorizeEnumMatchesAppCategories(t *testing.T) {
	schema := categorizeSchema(CategorizeEnum)
	if schema.Properties["sugestoes"].MaxItems != nil {
		t.Error("MaxItems neste esquema faz o Gemini responder 400")
	}
	field := schema.Properties["sugestoes"].Items.Properties["categoria"]
	if field.Format != "enum" {
		t.Errorf("o Gemini recusa (400) um enum de texto sem format \"enum\": %q", field.Format)
	}
	enum := field.Enum
	if len(enum) != 9 || enum[0] != "FOOD" || enum[8] != "EDUCATION" {
		t.Errorf("enum: %v", enum)
	}
}
