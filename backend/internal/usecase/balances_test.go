package usecase

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

var balNow = time.Date(2026, 10, 2, 9, 12, 0, 0, time.UTC)

func fp(v float64) *float64 { return &v }
func isoDay(s string) *time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return &t
}

func bank(inst *uuid.UUID, item, name string, balance float64, updated time.Time) ports.Account {
	return ports.Account{ID: uuid.New(), ItemID: item, InstitutionID: inst, Type: "BANK", Name: name, Last4: "1234", Balance: balance, UpdatedAt: updated}
}

func card(inst *uuid.UUID, item, name string, invoice float64, limit, avail *float64, due *time.Time) ports.Account {
	return ports.Account{ID: uuid.New(), ItemID: item, InstitutionID: inst, Type: "CREDIT", Name: name, Last4: "9012", Balance: invoice,
		CreditLimit: limit, AvailableCreditLimit: avail, DueDate: due, UpdatedAt: balNow.Add(-time.Hour)}
}

func TestBuildBalances_TotalsAndShares(t *testing.T) {
	sID, iID := uuid.New(), uuid.New()
	insts := []ports.Institution{
		{ID: sID, ItemID: "s", Name: "Santander", Color: "ec0000", HasLogo: true},
		{ID: iID, ItemID: "i", Name: "Itaú", Color: "ec7000"},
	}
	accts := []ports.Account{
		bank(&sID, "s", "Santander", 8000, balNow.Add(-time.Hour)),
		bank(&iID, "i", "Itaú", 2000, balNow.Add(-2*time.Hour)),
		card(&sID, "s", "SANTANDER ELITE MASTER", 1200, fp(10000), fp(5800), isoDay("2026-10-10")),
		card(&iID, "i", "Click MC", 940, fp(1000), fp(220), isoDay("2026-10-04")),
	}
	v := BuildBalances(accts, insts, balNow)

	if v.TotalInAccount == nil || *v.TotalInAccount != 10000 {
		t.Fatalf("total = %v", v.TotalInAccount)
	}
	if v.OpenInvoices != 2140 {
		t.Errorf("faturas em aberto = %v", v.OpenInvoices)
	}
	if len(v.Institutions) != 2 || v.Institutions[0].Name != "Santander" {
		t.Fatalf("ordem: %+v", v.Institutions)
	}
	s, i := v.Institutions[0], v.Institutions[1]
	if s.Share != 0.8 || i.Share != 0.2 || !s.HasLogo || i.HasLogo {
		t.Errorf("participação/logo: %v %v", s.Share, i.Share)
	}
	if i.Accounts[0].Name != "Conta corrente" {
		t.Errorf("nome da conta igual ao do banco deveria virar 'Conta corrente': %q", i.Accounts[0].Name)
	}
	if got := s.Cards[0].Name; got != "Elite Master" {
		t.Errorf("título do cartão = %q", got)
	}
	if !v.AsOf.Equal(balNow.Add(-2 * time.Hour)) {
		t.Errorf("asOf deveria ser a atualização mais antiga: %v", v.AsOf)
	}
}

func TestBuildBalances_NegativeBankHasNoShare(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	v := BuildBalances([]ports.Account{
		bank(&a, "a", "A", -300, balNow), bank(&b, "b", "B", 700, balNow),
	}, []ports.Institution{{ID: a, Name: "A"}, {ID: b, Name: "B"}}, balNow)
	if *v.TotalInAccount != 400 {
		t.Errorf("o negativo entra com sinal no total: %v", *v.TotalInAccount)
	}
	for _, in := range v.Institutions {
		if in.Name == "A" && in.Share != 0 {
			t.Errorf("banco negativo sem fatia: %v", in.Share)
		}
		if in.Name == "B" && in.Share != 1 {
			t.Errorf("a fatia é sobre os saldos positivos: %v", in.Share)
		}
	}
}

func TestBuildBalances_WithoutInstitutionGroupsByItem(t *testing.T) {
	v := BuildBalances([]ports.Account{
		bank(nil, "item-x", "Banco Antigo", 100, balNow),
		card(nil, "item-x", "Cartão Antigo", 10, nil, nil, nil),
		bank(nil, "item-y", "Outro", 5, balNow),
	}, nil, balNow)
	if len(v.Institutions) != 2 {
		t.Fatalf("esperava 2 grupos: %+v", v.Institutions)
	}
	in := v.Institutions[0]
	if in.Name != "Banco Antigo" || len(in.Accounts) != 1 || len(in.Cards) != 1 || in.HasLogo || in.Color != "" {
		t.Errorf("grupo inesperado: %+v", in)
	}
	if strings.Contains(in.ID, "item-x") || in.ID == "item-"+"x" {
		t.Errorf("o item_id não pode vazar no id: %q", in.ID)
	}
	if v.Institutions[1].ID == in.ID {
		t.Error("ids deveriam ser distintos")
	}
}

func TestBuildBalances_NoBankAccount(t *testing.T) {
	v := BuildBalances([]ports.Account{card(nil, "i", "Cartão", 50, nil, nil, nil)}, nil, balNow)
	if v.TotalInAccount != nil {
		t.Errorf("sem conta corrente o total é desconhecido, não zero: %v", *v.TotalInAccount)
	}
	if c := v.Institutions[0].Cards[0]; c.Used != nil || c.UsedRatio != nil || c.UsageLevel != LevelOK {
		t.Errorf("cartão sem limite: %+v", c)
	}
	if len(BuildBalances(nil, nil, balNow).Institutions) != 0 {
		t.Error("sem contas, sem instituições")
	}
}

func TestUsageLevel_Boundaries(t *testing.T) {
	for ratio, want := range map[float64]Level{0: LevelOK, 0.699: LevelOK, 0.7: LevelWarning, 0.899: LevelWarning, 0.9: LevelCritical, 1: LevelCritical} {
		if got := UsageLevel(ratio); got != want {
			t.Errorf("UsageLevel(%v) = %s, quer %s", ratio, got, want)
		}
	}
}

func TestDueLevel_Boundaries(t *testing.T) {
	cases := []struct {
		invoice float64
		days    int
		want    Level
	}{
		{100, 4, LevelOK}, {100, 3, LevelWarning}, {100, 0, LevelWarning}, {100, -1, LevelOK}, {100, -22, LevelOK},
		{0, -5, LevelOK}, {0, 1, LevelOK}, {-50, -1, LevelOK},
	}
	for _, c := range cases {
		if got := DueLevel(c.invoice, c.days); got != c.want {
			t.Errorf("DueLevel(%v, %d) = %s, quer %s", c.invoice, c.days, got, c.want)
		}
	}
}

func TestDaysUntil_UsesCalendarDays(t *testing.T) {
	brt := time.FixedZone("BRT", -3*3600)
	// 23:30 em Brasília de 2/10 já é 3/10 em UTC; o dia civil do usuário é o que conta.
	late := time.Date(2026, 10, 2, 23, 30, 0, 0, brt)
	if got := DaysUntil(late, *isoDay("2026-10-10")); got != 8 {
		t.Errorf("got %d, quer 8", got)
	}
	if got := DaysUntil(late, *isoDay("2026-10-02")); got != 0 {
		t.Errorf("hoje = 0, got %d", got)
	}
	if got := DaysUntil(late, *isoDay("2026-10-01")); got != -1 {
		t.Errorf("ontem = -1, got %d", got)
	}
}

func TestBuildBalances_CardLimitEdgeCases(t *testing.T) {
	v := BuildBalances([]ports.Account{
		card(nil, "i", "A", 100, fp(1000), fp(300), nil),                 // 70%
		card(nil, "i", "B", 100, fp(1000), fp(100), nil),                 // 90%
		card(nil, "i", "C", 100, fp(1000), fp(-50), nil),                 // estourado
		card(nil, "i", "D", 100, fp(0), fp(0), nil),                      // limite zero
		card(nil, "i", "E", 100, fp(1000), nil, nil),                     // disponível ausente
		card(nil, "i", "F", 0, fp(1000), fp(1000), isoDay("2026-10-01")), // sem fatura, vencimento passado
	}, nil, balNow)
	byName := map[string]BalanceCard{}
	for _, c := range v.Institutions[0].Cards {
		byName[c.Name] = c
	}
	if byName["A"].UsageLevel != LevelWarning || byName["B"].UsageLevel != LevelCritical {
		t.Errorf("limites exatos: %s %s", byName["A"].UsageLevel, byName["B"].UsageLevel)
	}
	if r := byName["C"].UsedRatio; r == nil || *r != 1 {
		t.Errorf("uso acima do limite fica em 100%%: %v", r)
	}
	if byName["D"].UsedRatio != nil || byName["E"].UsedRatio != nil {
		t.Error("limite zero ou disponível ausente = sem barra")
	}
	if byName["F"].DueLevel != LevelOK {
		t.Errorf("sem fatura não há o que pagar: %s", byName["F"].DueLevel)
	}
}

func TestBuildBalances_Staleness(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	v := BuildBalances([]ports.Account{
		bank(&a, "a", "Velho", 1, balNow.Add(-36*time.Hour)),
		bank(&b, "b", "Novo", 1, balNow.Add(-36*time.Hour+time.Minute)),
	}, []ports.Institution{{ID: a, Name: "Velho"}, {ID: b, Name: "Novo"}}, balNow)
	for _, in := range v.Institutions {
		if in.Name == "Velho" && in.Stale {
			t.Error("exatamente 36 h ainda não é desatualizado")
		}
	}
	v = BuildBalances([]ports.Account{bank(&a, "a", "Velho", 1, balNow.Add(-36*time.Hour-time.Minute))}, []ports.Institution{{ID: a, Name: "Velho"}}, balNow)
	if !v.Institutions[0].Stale {
		t.Error("mais de 36 h deveria ser desatualizado")
	}
}

func TestCardTitle(t *testing.T) {
	for _, c := range [][3]string{
		{"SANTANDER ELITE MASTER", "Santander", "Elite Master"},
		{"Santander", "Santander", "Santander"},
		{"Click MC Platinum", "Itaú", "Click MC Platinum"},
		{"", "Itaú", "Cartão"},
		{"ÁGUA AZUL", "X", "Água Azul"},
	} {
		if got := cardTitle(c[0], c[1]); got != c[2] {
			t.Errorf("cardTitle(%q, %q) = %q, quer %q", c[0], c[1], got, c[2])
		}
	}
}

func TestFormatBalances(t *testing.T) {
	sID, iID := uuid.New(), uuid.New()
	insts := []ports.Institution{{ID: sID, Name: "Santander"}, {ID: iID, Name: "Itaú"}}
	auto := 900.0
	itau := bank(&iID, "i", "Conta", 4345.67, balNow.Add(-3*time.Hour))
	itau.AutoInvested = &auto
	text := FormatBalances(BuildBalances([]ports.Account{
		bank(&sID, "s", "Santander", 8000, balNow.Add(-3*time.Hour)), itau,
		card(&sID, "s", "SANTANDER ELITE", 1200, fp(10000), fp(5800), isoDay("2026-10-10")),
		card(&iID, "i", "Click", 940, fp(1000), fp(220), isoDay("2026-10-04")),
	}, insts, balNow), balNow)

	for _, want := range []string{
		"*Em conta: R$ 12.345,67*", "Santander  R$ 8.000,00", "65%", "35%",
		"Aplicado automaticamente: R$ 900,00", "Elite ·· 9012", "vence 10/10 (em 8 dias)",
		"⚠️ vence em 2 dias", "Limite usado ▓▓▓▓▓▓▓▓░░ 78% ⚠️ atenção", "disponível R$ 5.800,00",
		"Saldo devedor dos cartões, à parte: R$ 2.140,00", "Atualizado há 3 h",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("faltou %q em:\n%s", want, text)
		}
	}
}

func TestFormatBalances_EmptyAndNoBank(t *testing.T) {
	if got := FormatBalances(BalancesView{}, balNow); !strings.Contains(got, "Open Finance") {
		t.Errorf("sem Open Finance: %q", got)
	}
	text := FormatBalances(BuildBalances([]ports.Account{card(nil, "i", "Cartão", 10, nil, nil, nil)}, nil, balNow), balNow)
	if strings.Contains(text, "Em conta") || !strings.Contains(text, "Limite não informado") {
		t.Errorf("só cartão: %s", text)
	}
}

func TestFormatBalances_StaleNegativeOverdueAndAsterisks(t *testing.T) {
	a := uuid.New()
	text := FormatBalances(BuildBalances([]ports.Account{
		bank(&a, "a", "B", -50, balNow.Add(-50*time.Hour)),
		card(&a, "a", "Meu *Cartão*", 300, fp(100), fp(5), isoDay("2026-09-30")),
	}, []ports.Institution{{ID: a, Name: "Ban*co"}}, balNow), balNow)
	for _, want := range []string{"negativo", "desatualizado há 2 dias", "🚨 crítico"} {
		if !strings.Contains(text, want) {
			t.Errorf("faltou %q em:\n%s", want, text)
		}
	}
	if strings.Contains(text, "vence") || strings.Contains(text, "vencida") {
		t.Errorf("vencimento já passado não deve aparecer:\n%s", text)
	}
	if strings.Count(text, "*") != 6 { // só os três negritos: Seus saldos, Em conta e Cartões
		t.Errorf("asteriscos de nomes do banco deveriam sair:\n%s", text)
	}
}

func TestFormatBalances_FitsTelegramLimit(t *testing.T) {
	a := uuid.New()
	accts := []ports.Account{bank(&a, "a", "B", 1, balNow)}
	for i := 0; i < 200; i++ {
		accts = append(accts, card(&a, "a", strings.Repeat("Cartão ", 5), 100, fp(1000), fp(500), isoDay("2026-10-20")))
	}
	text := FormatBalances(BuildBalances(accts, []ports.Institution{{ID: a, Name: "B"}}, balNow), balNow)
	if n := utf8.RuneCountInString(text); n > 4000 {
		t.Errorf("mensagem com %d caracteres", n)
	}
	if !strings.Contains(text, "…e mais ") || !strings.Contains(text, "/sync") {
		t.Errorf("deveria cortar com 'e mais N' e manter o rodapé:\n%s", text[len(text)-200:])
	}
}

// No Meu Pluggy todas as conexões trazem o conector "MeuPluggy": o banco vem do nome das contas.
func TestBuildBalances_AggregatorConnectorUsesAccountNames(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	insts := []ports.Institution{
		{ID: a, ItemID: "a", Name: "MeuPluggy", Color: "ef294b", HasLogo: true},
		{ID: b, ItemID: "b", Name: "MeuPluggy", Color: "ef294b", HasLogo: true},
	}
	v := BuildBalances([]ports.Account{
		bank(&a, "a", "itau", 100, balNow), bank(&b, "b", "Banco Santander", 50, balNow),
	}, insts, balNow)
	got := map[string]InstitutionBalance{}
	for _, in := range v.Institutions {
		got[in.Name] = in
	}
	if got["Itaú"].Color != "ec7000" || got["Santander"].Color != "ec0000" || got["Itaú"].HasLogo {
		t.Errorf("bancos: %+v", v.Institutions)
	}
}

func TestBuildBalances_AutoInvestedEqualToBalanceIsHidden(t *testing.T) {
	a, b := bank(nil, "x", "Itaú", 100, balNow), bank(nil, "y", "Outro", 100, balNow)
	a.AutoInvested, b.AutoInvested = fp(100), fp(40)
	v := BuildBalances([]ports.Account{a, b}, nil, balNow)
	for _, in := range v.Institutions {
		got := in.Accounts[0].AutoInvested
		if in.Name == "Itaú" && got != nil {
			t.Errorf("igual ao saldo não deve aparecer: %v", *got)
		}
		if in.Name == "Outro" && (got == nil || *got != 40) {
			t.Errorf("valor próprio deve aparecer: %v", got)
		}
	}
}

func TestBuildBalances_AccountNamedLikeBankWithoutAccent(t *testing.T) {
	a := uuid.New()
	v := BuildBalances([]ports.Account{bank(&a, "a", "itau", 1, balNow)}, []ports.Institution{{ID: a, Name: "MeuPluggy"}}, balNow)
	if got := v.Institutions[0].Accounts[0].Name; got != "Conta corrente" {
		t.Errorf("nome = %q", got)
	}
}
