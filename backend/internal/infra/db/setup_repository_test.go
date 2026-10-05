package db

import (
	"context"
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/setup"
)

// newSetupStore usa o banco descartável dos testes. Se encontrar um setup concluído, para: parece o banco de uso.
func newSetupStore(t *testing.T) *SetupStore {
	t.Helper()
	_, pg := newTestRepo(t)
	ctx := context.Background()
	var done bool
	if err := pg.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM dashboard_owner WHERE setup_completed_at IS NOT NULL)`).Scan(&done); err != nil {
		t.Fatal(err)
	}
	if done {
		t.Skip("dashboard_owner com setup concluído: TEST_DATABASE_URL parece o banco de uso")
	}
	reset := func() {
		pg.Pool.Exec(ctx, `DELETE FROM dashboard_owner`)                                                             //nolint:errcheck
		pg.Pool.Exec(ctx, `DELETE FROM setup_draft`)                                                                 //nolint:errcheck
		pg.Pool.Exec(ctx, `DELETE FROM settings WHERE key IN ('CHANNEL', 'TELEGRAM_BOT_TOKEN', 'TELEGRAM_CHAT_ID')`) //nolint:errcheck
	}
	reset()
	t.Cleanup(reset)
	return NewSetupStore(pg)
}

func TestSetupStore_DraftRoundTrip(t *testing.T) {
	s := newSetupStore(t)
	ctx := context.Background()
	if d, err := s.LoadDraft(ctx); err != nil || d.TelegramToken != "" || d.Candidate != nil {
		t.Fatalf("rascunho vazio: %+v %v", d, err)
	}
	want := setup.Draft{PasswordHash: "$argon2id$v=19$x", TelegramToken: "cifrado", TelegramBot: "meu_bot", TelegramOffset: 99,
		Candidate: &setup.Candidate{ID: 42, Name: "Dono", Username: "dono"}, CandidateAccepted: true}
	if err := s.SaveDraft(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadDraft(ctx)
	if err != nil || got.PasswordHash != want.PasswordHash || got.TelegramToken != "cifrado" || got.TelegramOffset != 99 ||
		got.Candidate == nil || *got.Candidate != *want.Candidate || !got.CandidateAccepted {
		t.Fatalf("rascunho lido: %+v %v", got, err)
	}
	want.Candidate, want.CandidateAccepted = nil, false
	s.SaveDraft(ctx, want) //nolint:errcheck
	if got, _ := s.LoadDraft(ctx); got.Candidate != nil {
		t.Fatal("\"não sou eu\" apaga o candidato")
	}
}

func TestSetupStore_CompleteOnceInTransaction(t *testing.T) {
	s := newSetupStore(t)
	ctx := context.Background()
	s.SaveDraft(ctx, setup.Draft{PasswordHash: "$argon2id$v=19$rascunho"}) //nolint:errcheck
	c := setup.Completion{
		PasswordHash: "$argon2id$v=19$nova",
		Rows:         []settings.Row{{Key: "CHANNEL", Value: "telegram"}, {Key: "TELEGRAM_CHAT_ID", Value: "42"}},
	}
	at, err := s.Complete(ctx, c)
	if err != nil || at == nil {
		t.Fatalf("concluir: %v %v", at, err)
	}
	if again, err := s.Complete(ctx, c); err != nil || again != nil {
		t.Fatalf("a mesma conclusão de novo não grava: %v %v", again, err)
	}
	owner, err := s.LoadOwner(ctx)
	if err != nil || owner.PasswordHash != "$argon2id$v=19$nova" || owner.CompletedAt == nil || !owner.CompletedAt.Equal(*at) {
		t.Fatalf("dono: %+v %v", owner, err)
	}
	rows, _ := NewSettingsStore(s.db).LoadSettings(ctx)
	found := map[string]string{}
	for _, r := range rows {
		found[r.Key] = r.Value
	}
	if found["CHANNEL"] != "telegram" || found["TELEGRAM_CHAT_ID"] != "42" {
		t.Fatalf("canal gravado: %v", found)
	}
	if d, _ := s.LoadDraft(ctx); d.PasswordHash != "" {
		t.Fatal("o rascunho é apagado ao concluir")
	}

	// Reaberto: espera o dono como está agora; mantém a senha (hash vazio) e marca a reabertura como usada.
	at2, err := s.Complete(ctx, setup.Completion{Reopen: true, Expected: owner})
	if err != nil || at2 == nil {
		t.Fatalf("concluir a reabertura: %v %v", at2, err)
	}
	owner, _ = s.LoadOwner(ctx)
	if owner.PasswordHash != "$argon2id$v=19$nova" || !owner.ReopenDone {
		t.Fatalf("reaberto: %+v", owner)
	}
	if err := s.ClearReopen(ctx); err != nil {
		t.Fatal(err)
	}
	if owner, _ = s.LoadOwner(ctx); owner.ReopenDone {
		t.Fatal("ClearReopen libera a próxima reabertura")
	}
}

// Uma linha de canal inválida desfaz tudo: nem o "concluído" fica gravado.
func TestSetupStore_CompleteRollsBack(t *testing.T) {
	s := newSetupStore(t)
	ctx := context.Background()
	_, err := s.Complete(ctx, setup.Completion{Rows: []settings.Row{{Key: "chave inválida", Value: "x"}}})
	if err == nil {
		t.Fatal("chave fora do padrão deveria falhar")
	}
	if owner, _ := s.LoadOwner(ctx); owner.CompletedAt != nil {
		t.Fatal("falha no meio não pode deixar o setup concluído")
	}
}
