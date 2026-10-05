// Package auth guarda os desafios do segundo fator: códigos de uso único enviados ao chat da pessoa.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// Kind separa os desafios do login, da confirmação nas Configurações e do setup: um código de um não vale no outro.
type Kind string

const (
	Login   Kind = "login"
	Confirm Kind = "confirm"
	// Setup confirma, no setup, que o chat que mandou /start é da pessoa.
	Setup Kind = "setup"
)

const (
	CodeDigits  = 6
	CodeTTL     = 5 * time.Minute
	MaxAttempts = 3
	// ResendAfter é o intervalo mínimo entre dois envios do mesmo tipo.
	ResendAfter = time.Minute
	// MaxSends é o teto de códigos enviados por SendWindow, somando todos os tipos (o app é de uma pessoa só).
	MaxSends   = 10
	SendWindow = time.Hour
)

var (
	ErrNoChallenge      = errors.New("código expirado; peça um novo")
	ErrWrongCode        = errors.New("código incorreto")
	ErrTooManyAttempts  = errors.New("muitos códigos errados; peça um novo")
	ErrTooSoon          = errors.New("aguarde um minuto para pedir outro código")
	ErrSendLimitReached = errors.New("muitos códigos pedidos; tente mais tarde")
)

type challenge struct {
	bind     string // a quem o código pertence: o navegador que acertou a senha (login) ou a sessão (confirmação)
	hash     [sha256.Size]byte
	expires  time.Time
	sentAt   time.Time
	attempts int
}

// Challenges mantém no máximo um desafio vivo por tipo, só em memória: reiniciar o app derruba todos.
type Challenges struct {
	mu      sync.Mutex
	now     func() time.Time
	newCode func() (string, error)
	live    map[Kind]*challenge
	sends   []time.Time
}

func NewChallenges() *Challenges { return newChallenges(time.Now, randomCode) }

func newChallenges(now func() time.Time, newCode func() (string, error)) *Challenges {
	return &Challenges{now: now, newCode: newCode, live: map[Kind]*challenge{}}
}

// Start cria um desafio do tipo, preso a bind, e devolve o código para ser enviado. Substitui o desafio anterior do
// mesmo tipo, respeitando o intervalo entre envios e o teto por hora.
func (c *Challenges) Start(kind Kind, bind string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if prev := c.live[kind]; prev != nil && now.Sub(prev.sentAt) < ResendAfter {
		return "", ErrTooSoon
	}
	recent := c.sends[:0]
	for _, t := range c.sends {
		if now.Sub(t) < SendWindow {
			recent = append(recent, t)
		}
	}
	c.sends = recent
	if len(c.sends) >= MaxSends {
		return "", ErrSendLimitReached
	}
	code, err := c.newCode()
	if err != nil {
		return "", err
	}
	c.sends = append(c.sends, now)
	c.live[kind] = &challenge{bind: bind, hash: sha256.Sum256([]byte(code)), expires: now.Add(CodeTTL), sentAt: now}
	return code, nil
}

// Cancel descarta o desafio do tipo (o envio falhou). O envio continua contando no teto.
func (c *Challenges) Cancel(kind Kind) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.live, kind)
}

// Verify confere o código. Acertando, o desafio some (uso único); errando MaxAttempts vezes, também.
func (c *Challenges) Verify(kind Kind, bind, code string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := c.live[kind]
	if ch == nil || ch.bind != bind || !c.now().Before(ch.expires) {
		return ErrNoChallenge
	}
	h := sha256.Sum256([]byte(code))
	if subtle.ConstantTimeCompare(h[:], ch.hash[:]) == 1 {
		delete(c.live, kind)
		return nil
	}
	ch.attempts++
	if ch.attempts >= MaxAttempts {
		delete(c.live, kind)
		return ErrTooManyAttempts
	}
	return ErrWrongCode
}

func randomCode() (string, error) {
	limit := big.NewInt(1)
	for range CodeDigits {
		limit.Mul(limit, big.NewInt(10))
	}
	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return "", fmt.Errorf("erro ao gerar código: %w", err)
	}
	return fmt.Sprintf("%0*d", CodeDigits, n), nil
}
