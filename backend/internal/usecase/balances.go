package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

const (
	// StaleAfter é o tempo sem atualizar a partir do qual o saldo é tido como desatualizado
	// (o Meu Pluggy atualiza cerca de uma vez por dia).
	StaleAfter = 36 * time.Hour

	cardWarnRatio     = 0.7
	cardCriticalRatio = 0.9
	dueSoonDays       = 3
)

// Level é a severidade de um alerta; a tela sempre mostra ícone e texto além da cor.
type Level string

const (
	LevelOK       Level = "ok"
	LevelWarning  Level = "warning"
	LevelCritical Level = "critical"
)

// BalanceAccount é uma conta corrente.
type BalanceAccount struct {
	ID           string
	Name         string
	Last4        string
	Balance      float64
	AutoInvested *float64 // aplicado automaticamente; fora do total em conta
}

// BalanceCard é um cartão de crédito. Cartões nunca são somados entre si.
type BalanceCard struct {
	ID             string
	Name           string
	Brand          string
	Last4          string
	Invoice        float64 // valor devido informado pelo banco
	Limit          *float64
	Available      *float64
	Used           *float64 // limite - disponível; nil sem limite informado
	UsedRatio      *float64 // 0 a 1
	UsageLevel     Level
	CloseDate      *time.Time
	DueDate        *time.Time
	DaysToDue      *int
	DueLevel       Level
	MinimumPayment *float64
}

// InstitutionBalance reúne o que é de um mesmo banco: contas correntes e cartões.
type InstitutionBalance struct {
	ID        string // opaco: o item_id do Pluggy nunca sai
	Name      string
	Color     string // hexadecimal de 6 dígitos sem "#"; vazio se desconhecida
	HasLogo   bool
	UpdatedAt time.Time
	Stale     bool
	Total     float64 // soma das contas correntes
	HasBank   bool    // tem conta corrente
	Share     float64 // participação no total em conta (0 a 1)
	Accounts  []BalanceAccount
	Cards     []BalanceCard
}

// BalancesView é a leitura única que alimenta o painel e o comando /saldos.
type BalancesView struct {
	AsOf           time.Time // a atualização mais antiga entre os bancos
	TotalInAccount *float64  // nil sem conta corrente sincronizada
	OpenInvoices   float64   // soma dos saldos devedores dos cartões, à parte do total em conta
	Institutions   []InstitutionBalance
}

// Balances monta os saldos por banco no instante now.
func (i *Insights) Balances(ctx context.Context, now time.Time) (BalancesView, error) {
	accounts, err := i.reader.Accounts(ctx)
	if err != nil {
		return BalancesView{}, err
	}
	institutions, err := i.reader.Institutions(ctx)
	if err != nil {
		return BalancesView{}, err
	}
	return BuildBalances(accounts, institutions, now), nil
}

// BuildBalances agrupa as contas por instituição (ou, sem instituição, por conexão), calcula
// totais, participação, uso do limite e vencimento, e marca o que está desatualizado.
func BuildBalances(accounts []domain.Account, institutions []domain.Institution, now time.Time) BalancesView {
	known := make(map[string]domain.Institution, len(institutions))
	for _, in := range institutions {
		known[in.ID.String()] = in
	}

	groups := map[string]*InstitutionBalance{}
	var order []string
	for _, a := range accounts {
		key := "item-" + shortHash(a.ItemID)
		var meta *domain.Institution
		if a.InstitutionID != nil {
			if in, ok := known[a.InstitutionID.String()]; ok {
				key, meta = in.ID.String(), &in
			}
		}
		if meta != nil && isAggregator(meta.Name) {
			// O conector é o agregador (Meu Pluggy), não o banco: nome, cor e logo vêm das próprias contas.
			meta = nil
		}
		g := groups[key]
		if g == nil {
			g = &InstitutionBalance{ID: key, UpdatedAt: a.UpdatedAt}
			if meta != nil {
				g.Name, g.Color, g.HasLogo = meta.Name, meta.Color, meta.HasLogo
			}
			groups[key] = g
			order = append(order, key)
		}
		if a.UpdatedAt.Before(g.UpdatedAt) {
			g.UpdatedAt = a.UpdatedAt
		}
		if g.Name == "" {
			g.Name = a.Name
			if meta == nil {
				g.Name, g.Color = knownBrand(a.Name)
			}
		}
		if a.Type == domain.AccountBank {
			g.HasBank = true
			g.Total += a.Balance
			g.Accounts = append(g.Accounts, BalanceAccount{ID: a.ID.String(), Name: a.Name, Last4: a.Last4, Balance: a.Balance, AutoInvested: autoInvested(a)})
		} else {
			g.Cards = append(g.Cards, buildCard(a, now))
		}
	}

	view := BalancesView{}
	var bank *float64
	var positive float64
	for _, key := range order {
		g := groups[key]
		g.Stale = now.Sub(g.UpdatedAt) > StaleAfter
		for n := range g.Accounts {
			if brand, _ := knownBrand(g.Accounts[n].Name); strings.EqualFold(g.Accounts[n].Name, g.Name) || brand == g.Name || g.Accounts[n].Name == "" {
				g.Accounts[n].Name = "Conta corrente"
			}
		}
		for n := range g.Cards {
			g.Cards[n].Name = cardTitle(g.Cards[n].Name, g.Name)
			if g.Cards[n].Invoice > 0 {
				view.OpenInvoices += g.Cards[n].Invoice
			}
		}
		slices.SortFunc(g.Cards, func(x, y BalanceCard) int { return strings.Compare(x.Name, y.Name) })
		if g.HasBank {
			sum := g.Total
			if bank != nil {
				sum += *bank
			}
			bank = &sum
			positive += max(0, g.Total)
		}
		if view.AsOf.IsZero() || g.UpdatedAt.Before(view.AsOf) {
			view.AsOf = g.UpdatedAt
		}
		view.Institutions = append(view.Institutions, *g)
	}
	view.TotalInAccount = bank

	// Banco negativo mostra o valor, mas não ganha fatia na barra.
	for n := range view.Institutions {
		if in := &view.Institutions[n]; in.HasBank && in.Total > 0 && positive > 0 {
			in.Share = in.Total / positive
		}
	}
	slices.SortStableFunc(view.Institutions, func(x, y InstitutionBalance) int {
		switch {
		case x.Total > y.Total:
			return -1
		case x.Total < y.Total:
			return 1
		}
		return strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name))
	})
	return view
}

// autoInvested devolve o aplicado automaticamente só quando ele diz algo além do saldo. Alguns bancos (Itaú e Santander
// pelo Meu Pluggy) informam o saldo inteiro da conta nesse campo: mostrar seria dizer que o dinheiro está fora da conta.
func autoInvested(a domain.Account) *float64 {
	if a.AutoInvested == nil || *a.AutoInvested <= 0 || math.Abs(*a.AutoInvested-a.Balance) < 0.005 {
		return nil
	}
	return a.AutoInvested
}

// isAggregator: no Meu Pluggy todas as conexões trazem o mesmo conector, com o nome e o logo do próprio Pluggy.
func isAggregator(name string) bool {
	n := strings.ToLower(strings.ReplaceAll(name, " ", ""))
	return n == "meupluggy" || n == "pluggy"
}

var knownBrands = []struct{ keyword, name, color string }{
	{"itaú", "Itaú", "ec7000"}, {"itau", "Itaú", "ec7000"}, {"santander", "Santander", "ec0000"},
	{"nubank", "Nubank", "8a05be"}, {"bradesco", "Bradesco", "cc092f"}, {"inter", "Inter", "ff7a00"},
	{"caixa", "Caixa", "005ca9"}, {"brasil", "Banco do Brasil", "fae128"}, {"c6", "C6 Bank", "242424"},
}

// knownBrand dá nome limpo e cor de marca a um banco conhecido pelo nome da conta; os demais mantêm o nome.
func knownBrand(accountName string) (name, color string) {
	lower := strings.ToLower(accountName)
	for _, b := range knownBrands {
		if strings.Contains(lower, b.keyword) {
			return b.name, b.color
		}
	}
	return accountName, ""
}

func buildCard(a domain.Account, now time.Time) BalanceCard {
	c := BalanceCard{
		ID: a.ID.String(), Name: a.Name, Brand: a.Brand, Last4: a.Last4, Invoice: a.Balance,
		Limit: a.CreditLimit, Available: a.AvailableCreditLimit, CloseDate: a.CloseDate, DueDate: a.DueDate,
		MinimumPayment: a.MinimumPayment, UsageLevel: LevelOK, DueLevel: LevelOK,
	}
	if a.CreditLimit != nil && a.AvailableCreditLimit != nil && *a.CreditLimit > 0 {
		used := max(0, *a.CreditLimit-*a.AvailableCreditLimit)
		ratio := min(1, used / *a.CreditLimit)
		c.Used, c.UsedRatio, c.UsageLevel = &used, &ratio, UsageLevel(ratio)
	}
	if a.DueDate != nil {
		days := DaysUntil(now, *a.DueDate)
		c.DaysToDue = &days
		c.DueLevel = DueLevel(c.Invoice, days)
	}
	return c
}

// UsageLevel: a partir de 70% do limite é atenção; a partir de 90%, crítico.
func UsageLevel(ratio float64) Level {
	switch {
	case ratio >= cardCriticalRatio:
		return LevelCritical
	case ratio >= cardWarnRatio:
		return LevelWarning
	}
	return LevelOK
}

// DueLevel: vencendo em até 3 dias é atenção. Data já passada não alerta: o banco costuma manter o vencimento da
// fatura que já foi paga enquanto o valor devido é o da fatura seguinte. Sem fatura, nada a pagar.
func DueLevel(invoice float64, daysToDue int) Level {
	switch {
	case invoice <= 0:
		return LevelOK
	case daysToDue >= 0 && daysToDue <= dueSoonDays:
		return LevelWarning
	}
	return LevelOK
}

// DaysUntil conta os dias entre o dia civil de now (no fuso dele) e o dia civil de due.
func DaysUntil(now, due time.Time) int {
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	dy, dm, dd := due.UTC().Date()
	return int(time.Date(dy, dm, dd, 0, 0, 0, 0, time.UTC).Sub(today).Hours() / 24)
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:16]
}

// cardTitle tira o nome do banco do começo ("SANTANDER ELITE MASTER" -> "Elite Master") e
// capitaliza nomes escritos todo em maiúsculas.
func cardTitle(name, institution string) string {
	name = strings.Join(strings.Fields(name), " ")
	if inst := strings.TrimSpace(institution); inst != "" && len(name) > len(inst) && strings.EqualFold(name[:len(inst)], inst) {
		if rest := strings.TrimSpace(name[len(inst):]); rest != "" {
			name = rest
		}
	}
	if name == "" {
		return "Cartão"
	}
	if name == strings.ToUpper(name) {
		prevLetter := false
		name = strings.Map(func(r rune) rune {
			out := unicode.ToLower(r)
			if !prevLetter {
				out = unicode.ToUpper(r)
			}
			prevLetter = unicode.IsLetter(r)
			return out
		}, name)
	}
	return name
}

// BalancesText é o comando /saldos: o mesmo cálculo do painel, em texto.
func (i *Insights) BalancesText(ctx context.Context, now time.Time) (string, error) {
	view, err := i.Balances(ctx, now)
	if err != nil {
		return "", err
	}
	return FormatBalances(view, now), nil
}

const maxBalancesRunes = 3600 // folga sob o limite de 4096 caracteres do Telegram

// FormatBalances escreve os saldos em texto, com *negrito* no padrão do bot. Nomes vêm do banco:
// o asterisco é removido para não quebrar o negrito (o escape de HTML é feito pelo canal).
func FormatBalances(v BalancesView, now time.Time) string {
	if len(v.Institutions) == 0 {
		return "ℹ️ Nenhuma conta sincronizada. Configure o Open Finance (veja a seção Open Finance no README)."
	}
	clean := func(s string) string { return strings.ReplaceAll(s, "*", "") }

	var head strings.Builder
	fmt.Fprintf(&head, "💰 *Seus saldos* · %s\n", now.Format("02/01 15:04"))
	if v.TotalInAccount != nil {
		fmt.Fprintf(&head, "\n*Em conta: %s*\n", FormatBRL(*v.TotalInAccount))
		for _, in := range v.Institutions {
			if !in.HasBank {
				continue
			}
			line := fmt.Sprintf("🏦 %s  %s", clean(in.Name), FormatBRL(in.Total))
			switch {
			case in.Total < 0:
				line += " ⚠️ negativo"
			case in.Share > 0:
				line += fmt.Sprintf("  %s %d%%", textBar(in.Share, '█'), percent(in.Share))
			}
			if in.Stale {
				line += fmt.Sprintf("\n   ⚠️ desatualizado há %s", ageText(now.Sub(in.UpdatedAt)))
			}
			head.WriteString(line + "\n")
			for _, a := range in.Accounts {
				if a.AutoInvested != nil && *a.AutoInvested > 0 {
					fmt.Fprintf(&head, "   Aplicado automaticamente: %s (fora do total)\n", FormatBRL(*a.AutoInvested))
				}
			}
		}
	}

	var cards []string
	for _, in := range v.Institutions {
		for _, c := range in.Cards {
			cards = append(cards, formatCardText(in, c, clean))
		}
	}

	var foot strings.Builder
	if v.OpenInvoices > 0 {
		fmt.Fprintf(&foot, "\nSaldo devedor dos cartões, à parte: %s\n", FormatBRL(v.OpenInvoices))
	}
	fmt.Fprintf(&foot, "🕘 Atualizado há %s · /sync para atualizar", ageText(now.Sub(v.AsOf)))

	var body strings.Builder
	if len(cards) > 0 {
		body.WriteString("\n💳 *Cartões*\n")
		budget := maxBalancesRunes - len([]rune(head.String())) - len([]rune(foot.String())) - 40
		shown := 0
		for _, c := range cards {
			if budget -= len([]rune(c)) + 1; budget < 0 {
				break
			}
			body.WriteString(c + "\n")
			shown++
		}
		if shown < len(cards) {
			fmt.Fprintf(&body, "…e mais %d\n", len(cards)-shown)
		}
	}
	return head.String() + body.String() + foot.String()
}

func formatCardText(in InstitutionBalance, c BalanceCard, clean func(string) string) string {
	title := clean(in.Name) + " " + clean(c.Name)
	if c.Last4 != "" {
		title += " ·· " + c.Last4
	}
	var b strings.Builder
	fmt.Fprintf(&b, "• %s\n  Saldo devedor %s", title, FormatBRL(c.Invoice))
	if c.DaysToDue != nil && *c.DaysToDue >= 0 && c.Invoice > 0 {
		switch c.DueLevel {
		case LevelWarning:
			fmt.Fprintf(&b, " · ⚠️ vence %s", dueText(*c.DaysToDue))
		default:
			fmt.Fprintf(&b, " · vence %s (%s)", c.DueDate.UTC().Format("02/01"), dueText(*c.DaysToDue))
		}
	}
	if c.UsedRatio == nil {
		b.WriteString("\n  Limite não informado pelo banco")
		return b.String()
	}
	fmt.Fprintf(&b, "\n  Limite usado %s %d%%", textBar(*c.UsedRatio, '▓'), percent(*c.UsedRatio))
	switch c.UsageLevel {
	case LevelCritical:
		b.WriteString(" 🚨 crítico")
	case LevelWarning:
		b.WriteString(" ⚠️ atenção")
	}
	if c.Available != nil {
		fmt.Fprintf(&b, " · disponível %s", FormatBRL(*c.Available))
	}
	return b.String()
}

func dueText(days int) string {
	switch days {
	case 0:
		return "hoje"
	case 1:
		return "amanhã"
	}
	return fmt.Sprintf("em %d dias", days)
}

func percent(ratio float64) int { return int(ratio*100 + 0.5) }

// textBar desenha uma barra de 10 posições; qualquer valor acima de zero mostra ao menos um bloco.
func textBar(ratio float64, fill rune) string {
	n := min(10, int(ratio*10+0.5))
	if ratio > 0 && n == 0 {
		n = 1
	}
	return strings.Repeat(string(fill), n) + strings.Repeat("░", 10-n)
}

func ageText(d time.Duration) string {
	switch hours := int(d.Hours()); {
	case d < time.Minute:
		return "instantes"
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case hours < 48:
		return fmt.Sprintf("%d h", hours)
	default:
		return fmt.Sprintf("%d dias", hours/24)
	}
}
