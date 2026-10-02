package usecase

import (
	"sort"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

const (
	// Matriz categoria x mês: quantos meses mostrar.
	ReviewMatrixMonths = 6

	// Aumento: a categoria precisa ter subido ao menos R$ 50 e 20% sobre a média dos até 3 meses anteriores.
	increaseBaselineMonths = 3
	minIncreaseAmount      = 50.0
	minIncreaseRatio       = 1.20

	// Gasto formiga: compras pequenas (fora o Pix, que mistura pessoas e comércio) repetidas na mesma conta.
	maxAntAmount = 40.0
	minAntCount  = 4
	minAntTotal  = 80.0
	// A economia é uma hipótese: cortar o hábito pela metade.
	antSavingShare = 0.5

	// Duplicata: mesma conta e mesmo valor, com até 3 dias de diferença.
	minDuplicateAmount = 5.0
	maxDuplicateDays   = 3

	// Novo: conta que só aparece no mês, havendo ao menos 2 meses anteriores com dados para comparar.
	minNewAmount        = 30.0
	minNewHistoryMonths = 2
)

// Tipos de sugestão da revisão.
const (
	ReviewIncrease  = "INCREASE"
	ReviewFixed     = "FIXED"
	ReviewAnt       = "ANT"
	ReviewDuplicate = "DUPLICATE"
	ReviewNew       = "NEW"
)

// ReviewKinds é a lista fechada de tipos (a mesma do CHECK da tabela review_dismissals).
var ReviewKinds = []string{ReviewIncrease, ReviewFixed, ReviewAnt, ReviewDuplicate, ReviewNew}

// ReviewRow é uma categoria na matriz, com o total de cada mês (do mais antigo para o mais novo).
type ReviewRow struct {
	Category string
	Values   []float64
}

// Candidate é uma sugestão de corte. Key é a conta (descrição normalizada) ou, nos aumentos, a categoria.
type Candidate struct {
	Kind      string
	Key       string
	Label     string
	Category  string
	Saving    float64 // economia estimada: por mês quando Recurring, senão uma vez só
	Recurring bool    // vale todo mês (dá para anualizar); falso em duplicata e conta nova
	Amount    float64 // quanto custou no mês (aumento: o total da categoria)
	Baseline  float64 // só nos aumentos: a média dos meses anteriores
	Count     int     // lançamentos no mês (formiga e duplicata)
	Months    int     // há quantos meses a conta se repete (fixas)
	Dismissed bool
}

type Review struct {
	Months     []time.Time
	Matrix     []ReviewRow
	Candidates []Candidate
}

// BuildReview roda os detectores sobre o mês to (primeiro dia). catMonths cobre os meses da matriz,
// keyRows a janela do orçamento (para reconhecer fixas e contas novas) e payments só as despesas de to.
func BuildReview(catMonths []ports.CategoryMonth, keyRows []ports.ExpenseKeyMonth, rules map[string]ports.ExpenseClass,
	payments []ports.ExpensePayment, dismissed []ports.Dismissal, to time.Time) Review {
	from := to.AddDate(0, -(ReviewMatrixMonths - 1), 0)
	budget := BuildBudget(keyRows, rules, to.AddDate(0, -(BudgetMonths-1), 0), to)

	r := Review{}
	for m := from; !m.After(to); m = m.AddDate(0, 1, 0) {
		r.Months = append(r.Months, m)
	}
	r.Matrix = reviewMatrix(catMonths, r.Months)

	r.Candidates = append(r.Candidates, increases(r.Matrix)...)
	r.Candidates = append(r.Candidates, fixedBills(budget)...)
	r.Candidates = append(r.Candidates, antSpending(payments, budget.Classes)...)
	r.Candidates = append(r.Candidates, duplicates(payments, budget.Classes)...)
	r.Candidates = append(r.Candidates, newBills(budget, keyRows, to)...)

	off := make(map[ports.Dismissal]bool, len(dismissed))
	for _, d := range dismissed {
		off[d] = true
	}
	for i := range r.Candidates {
		c := &r.Candidates[i]
		c.Dismissed = off[ports.Dismissal{Kind: c.Kind, Key: c.Key}]
	}
	// As que se repetem todo mês primeiro, pelo custo no ano; as avulsas depois, pelo valor.
	sort.SliceStable(r.Candidates, func(i, j int) bool {
		a, b := r.Candidates[i], r.Candidates[j]
		if a.Recurring != b.Recurring {
			return a.Recurring
		}
		return a.Saving > b.Saving
	})
	return r
}

// BudgetMonths é a janela (em meses) do orçamento: a usada para reconhecer contas que se repetem.
const BudgetMonths = 12

func reviewMatrix(catMonths []ports.CategoryMonth, months []time.Time) []ReviewRow {
	index := make(map[time.Time]int, len(months))
	for i, m := range months {
		index[m] = i
	}
	byCat := map[string][]float64{}
	for _, c := range catMonths {
		i, ok := index[c.Month]
		if !ok {
			continue
		}
		if byCat[c.Category] == nil {
			byCat[c.Category] = make([]float64, len(months))
		}
		byCat[c.Category][i] += c.Total
	}
	rows := make([]ReviewRow, 0, len(byCat))
	for cat, values := range byCat {
		rows = append(rows, ReviewRow{Category: cat, Values: values})
	}
	sort.Slice(rows, func(i, j int) bool {
		if a, b := sum(rows[i].Values), sum(rows[j].Values); a != b {
			return a > b
		}
		return rows[i].Category < rows[j].Category
	})
	return rows
}

func sum(values []float64) float64 {
	var t float64
	for _, v := range values {
		t += v
	}
	return t
}

// increases compara o último mês de cada categoria com a média dos meses anteriores que têm dados.
func increases(matrix []ReviewRow) []Candidate {
	if len(matrix) == 0 {
		return nil
	}
	last := len(matrix[0].Values) - 1
	// Meses sem nenhuma despesa são falta de dado (antes da primeira sincronização), não gasto zero.
	var history []int
	for i := last - 1; i >= 0 && len(history) < increaseBaselineMonths; i-- {
		for _, row := range matrix {
			if row.Values[i] > 0 {
				history = append(history, i)
				break
			}
		}
	}
	if len(history) == 0 {
		return nil
	}
	var out []Candidate
	for _, row := range matrix {
		var total float64
		for _, i := range history {
			total += row.Values[i]
		}
		avg, cur := total/float64(len(history)), row.Values[last]
		if cur-avg >= minIncreaseAmount && cur >= avg*minIncreaseRatio {
			out = append(out, Candidate{Kind: ReviewIncrease, Key: row.Category, Category: row.Category, Saving: cur - avg, Recurring: true, Amount: cur, Baseline: avg})
		}
	}
	return out
}

// fixedBills lista as contas fixas do mês: o custo anual é o valor vezes 12.
func fixedBills(b Budget) []Candidate {
	var out []Candidate
	for _, it := range b.Items {
		if it.Class == ports.ClassFixed {
			out = append(out, Candidate{Kind: ReviewFixed, Key: it.Key, Label: it.Label, Category: it.Category, Saving: it.Total, Recurring: true, Amount: it.Total, Months: it.Months})
		}
	}
	return out
}

// antSpending soma as compras pequenas e frequentes de uma mesma conta variável.
func antSpending(payments []ports.ExpensePayment, classes map[string]ports.ExpenseClass) []Candidate {
	type acc struct {
		label, category string
		total           float64
		count           int
	}
	byKey := map[string]*acc{}
	for _, p := range payments {
		if p.Amount > maxAntAmount || p.PaymentMethod == "PIX" || classes[p.Key] != ports.ClassVariable {
			continue
		}
		a := byKey[p.Key]
		if a == nil {
			a = &acc{label: p.Label, category: p.Category}
			byKey[p.Key] = a
		}
		a.total += p.Amount
		a.count++
	}
	var out []Candidate
	for key, a := range byKey {
		if a.count >= minAntCount && a.total >= minAntTotal {
			out = append(out, Candidate{Kind: ReviewAnt, Key: key, Label: a.label, Category: a.category, Saving: a.total * antSavingShare, Recurring: true, Amount: a.total, Count: a.count})
		}
	}
	return out
}

// duplicates acha cobranças de mesmo valor na mesma conta com poucos dias de diferença.
// payments vem em ordem de data.
func duplicates(payments []ports.ExpensePayment, classes map[string]ports.ExpenseClass) []Candidate {
	type id struct {
		key    string
		amount float64
	}
	type run struct {
		first ports.ExpensePayment
		last  time.Time
		count int
	}
	open := map[id]*run{}
	var found []*run
	for _, p := range payments {
		if p.Amount < minDuplicateAmount || classes[p.Key] == ports.ClassInstallment {
			continue
		}
		k := id{p.Key, p.Amount}
		if r := open[k]; r != nil && p.Date.Sub(r.last) <= maxDuplicateDays*24*time.Hour {
			r.count++
			r.last = p.Date
			continue
		}
		r := &run{first: p, last: p.Date, count: 1}
		open[k] = r
		found = append(found, r)
	}
	var out []Candidate
	for _, r := range found {
		if r.count > 1 {
			out = append(out, Candidate{Kind: ReviewDuplicate, Key: r.first.Key, Label: r.first.Label, Category: r.first.Category,
				Saving: r.first.Amount * float64(r.count-1), Amount: r.first.Amount, Count: r.count})
		}
	}
	return out
}

// newBills lista as contas que só aparecem no mês escolhido.
func newBills(b Budget, keyRows []ports.ExpenseKeyMonth, to time.Time) []Candidate {
	history := map[time.Time]bool{}
	for _, r := range keyRows {
		if r.Month.Before(to) {
			history[r.Month] = true
		}
	}
	if len(history) < minNewHistoryMonths {
		return nil
	}
	var out []Candidate
	for _, it := range b.Items {
		if it.Months == 1 && it.Total >= minNewAmount && it.Class != ports.ClassInstallment {
			out = append(out, Candidate{Kind: ReviewNew, Key: it.Key, Label: it.Label, Category: it.Category, Saving: it.Total, Amount: it.Total, Count: it.Count})
		}
	}
	return out
}
