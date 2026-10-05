package auth

import (
	"errors"
	"regexp"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func setup() (*Challenges, *clock) {
	clk := &clock{t: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)}
	return newChallenges(clk.now, func() (string, error) { return "123456", nil }), clk
}

func TestVerifyAcceptsOnceOnly(t *testing.T) {
	c, _ := setup()
	code, err := c.Start(Login, "b")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Verify(Login, "b", code); err != nil {
		t.Fatalf("primeiro uso: %v", err)
	}
	if err := c.Verify(Login, "b", code); !errors.Is(err, ErrNoChallenge) {
		t.Fatalf("segundo uso: %v", err)
	}
}

func TestVerifyRejectsOtherBindAndKind(t *testing.T) {
	c, _ := setup()
	code, _ := c.Start(Login, "b")
	if err := c.Verify(Login, "outro", code); !errors.Is(err, ErrNoChallenge) {
		t.Fatalf("outro navegador: %v", err)
	}
	if err := c.Verify(Confirm, "b", code); !errors.Is(err, ErrNoChallenge) {
		t.Fatalf("outro tipo: %v", err)
	}
}

func TestVerifyExpires(t *testing.T) {
	c, clk := setup()
	code, _ := c.Start(Login, "b")
	clk.advance(CodeTTL)
	if err := c.Verify(Login, "b", code); !errors.Is(err, ErrNoChallenge) {
		t.Fatalf("expirado: %v", err)
	}
}

func TestVerifyKillsAfterMaxAttempts(t *testing.T) {
	c, _ := setup()
	code, _ := c.Start(Login, "b")
	for i := 1; i < MaxAttempts; i++ {
		if err := c.Verify(Login, "b", "000000"); !errors.Is(err, ErrWrongCode) {
			t.Fatalf("tentativa %d: %v", i, err)
		}
	}
	if err := c.Verify(Login, "b", "000000"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("última tentativa: %v", err)
	}
	if err := c.Verify(Login, "b", code); !errors.Is(err, ErrNoChallenge) {
		t.Fatalf("código certo depois de esgotar: %v", err)
	}
}

func TestStartResendAndLimit(t *testing.T) {
	c, clk := setup()
	if _, err := c.Start(Login, "b"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Start(Login, "b"); !errors.Is(err, ErrTooSoon) {
		t.Fatalf("reenvio imediato: %v", err)
	}
	// Outro tipo não espera o intervalo do login.
	if _, err := c.Start(Confirm, "s"); err != nil {
		t.Fatalf("confirmação: %v", err)
	}
	for i := 2; i < MaxSends; i++ {
		clk.advance(ResendAfter)
		if _, err := c.Start(Login, "b"); err != nil {
			t.Fatalf("envio %d: %v", i+1, err)
		}
	}
	clk.advance(ResendAfter)
	if _, err := c.Start(Login, "b"); !errors.Is(err, ErrSendLimitReached) {
		t.Fatalf("acima do teto: %v", err)
	}
	clk.advance(SendWindow)
	if _, err := c.Start(Login, "b"); err != nil {
		t.Fatalf("depois da janela: %v", err)
	}
}

func TestCancelDropsChallenge(t *testing.T) {
	c, _ := setup()
	code, _ := c.Start(Login, "b")
	c.Cancel(Login)
	if err := c.Verify(Login, "b", code); !errors.Is(err, ErrNoChallenge) {
		t.Fatalf("depois de cancelar: %v", err)
	}
}

func TestRandomCodeFormat(t *testing.T) {
	re := regexp.MustCompile(`^\d{6}$`)
	for range 50 {
		code, err := randomCode()
		if err != nil || !re.MatchString(code) {
			t.Fatalf("código %q, erro %v", code, err)
		}
	}
}
