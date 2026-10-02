package coach

import (
	"strconv"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// MaxCategorizeItems limita quantas contas vão à IA de uma vez (controle de custo e de exposição).
const MaxCategorizeItems = 40

// AICategories são as categorias que a IA pode sugerir. Pessoas e Outros ficam de fora: quem recebe um Pix
// só a pessoa sabe, e "não sei" é omitir a conta.
var AICategories = []domain.Category{
	domain.CategoryFood, domain.CategoryMarket, domain.CategoryTransport, domain.CategoryHealth,
	domain.CategoryEntertainment, domain.CategoryShopping, domain.CategoryHousing, domain.CategoryBills, domain.CategoryEducation,
}

// CategorizeContext é o que vai à IA para sugerir categorias: nomes de estabelecimentos já limpos, com
// quantidade e total. Pix, TED e transferências nunca vão (o nome seria de uma pessoa).
type CategorizeContext struct {
	Items []CategorizeItem `json:"contas"`
}

type CategorizeItem struct {
	ID    string  `json:"id"`
	Name  string  `json:"nome"`
	Count int     `json:"lancamentos"`
	Total float64 `json:"total_gasto"`
	Last  string  `json:"ultimo_mes"`
}

// IsPersonTransfer diz se a descrição é de Pix, TED ou transferência.
func IsPersonTransfer(label string) bool { return transferRE.MatchString(label) }

// BuildCategorizeContext escolhe as contas que podem ir à IA (sem transferências, as MaxCategorizeItems maiores) e
// devolve também essas contas, na ordem dos IDs "c1", "c2"...
func BuildCategorizeContext(groups []domain.UncategorizedGroup) (CategorizeContext, []domain.UncategorizedGroup) {
	c := CategorizeContext{Items: []CategorizeItem{}}
	var kept []domain.UncategorizedGroup
	for _, g := range groups {
		if IsPersonTransfer(g.Label) || len(kept) == MaxCategorizeItems {
			continue
		}
		kept = append(kept, g)
		c.Items = append(c.Items, CategorizeItem{ID: "c" + strconv.Itoa(len(kept)), Name: CoachLabel(g.Label), Count: g.Count, Total: g.Total, Last: g.Last.Format("2006-01")})
	}
	return c, kept
}

// CategoryChoice é a categoria sugerida pela IA para uma conta (Key é a descrição normalizada).
type CategoryChoice struct {
	Key      string
	Category string
}

// ValidateCategories fica só com o que a IA devolveu de aproveitável: IDs enviados, categorias permitidas, uma por conta.
func ValidateCategories(raw []ports.CategorySuggestion, kept []domain.UncategorizedGroup) []CategoryChoice {
	allowed := map[string]bool{}
	for _, c := range AICategories {
		allowed[string(c)] = true
	}
	seen := map[string]bool{}
	out := []CategoryChoice{}
	for _, s := range raw {
		i, ok := coachIndex(s.ID, "c", len(kept))
		if !ok || !allowed[s.Category] || seen[s.ID] {
			continue
		}
		seen[s.ID] = true
		out = append(out, CategoryChoice{Key: kept[i].Key, Category: s.Category})
	}
	return out
}
