package usecase

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func group(key, label string, n int, total float64) domain.UncategorizedGroup {
	return domain.UncategorizedGroup{Key: key, Label: label, Count: n, Total: total, Last: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)}
}

func TestBuildCategorizeContext(t *testing.T) {
	groups := []domain.UncategorizedGroup{
		group("pix enviado fulano", "Pix enviado Fulano de Tal", 10, 5000), // pessoa: nunca vai
		group("padaria do ze", "PADARIA DO ZE 123456", 8, 400),
		group("transferencia ciclana", "Transferência enviada Ciclana", 2, 300),
		group("loja x", "LOJA X (1/3)", 1, 90),
	}
	c, kept := BuildCategorizeContext(groups)

	if len(c.Items) != 2 || len(kept) != 2 || kept[0].Key != "padaria do ze" || kept[1].Key != "loja x" {
		t.Fatalf("só comércio, na ordem recebida: %+v %+v", c.Items, kept)
	}
	if it := c.Items[0]; it.ID != "c1" || it.Name != "PADARIA DO ZE" || it.Count != 8 || it.Total != 400 || it.Last != "2026-09" || c.Items[1].ID != "c2" || c.Items[1].Name != "LOJA X" {
		t.Errorf("itens sanitizados: %+v", c.Items)
	}
	raw, _ := json.Marshal(c)
	for _, leaked := range []string{"Fulano", "Ciclana", "123456", "pix"} {
		if strings.Contains(strings.ToLower(string(raw)), strings.ToLower(leaked)) {
			t.Errorf("o contexto enviado não pode conter %q: %s", leaked, raw)
		}
	}
}

func TestBuildCategorizeContext_Limit(t *testing.T) {
	var groups []domain.UncategorizedGroup
	for i := 0; i < 80; i++ {
		groups = append(groups, group("loja "+strings.Repeat("a", i%5), "Loja", 1, 10))
	}
	c, kept := BuildCategorizeContext(groups)
	if len(c.Items) != MaxCategorizeItems || len(kept) != MaxCategorizeItems {
		t.Errorf("limite de contas: %d", len(c.Items))
	}
}

func TestValidateCategories(t *testing.T) {
	kept := []domain.UncategorizedGroup{group("a", "A", 1, 1), group("b", "B", 1, 1), group("c", "C", 1, 1)}
	raw := []ports.CategorySuggestion{
		{ID: "c1", Category: "FOOD"},
		{ID: "c1", Category: "MARKET"},  // repetida: vale a primeira
		{ID: "c9", Category: "FOOD"},    // id que não foi enviado
		{ID: "c2", Category: "PEOPLE"},  // a IA não decide pessoas
		{ID: "c3", Category: "OTHER"},   // nem Outros: omitir é a resposta certa
		{ID: "x1", Category: "FOOD"},    // formato inválido
		{ID: "c3", Category: "HOUSING"}, // c3 ainda não foi aceita
		{ID: "c2", Category: "banana"},
	}
	got := ValidateCategories(raw, kept)
	if len(got) != 2 || got[0] != (CategoryChoice{"a", "FOOD"}) || got[1] != (CategoryChoice{"c", "HOUSING"}) {
		t.Errorf("sugestões válidas: %+v", got)
	}
}
