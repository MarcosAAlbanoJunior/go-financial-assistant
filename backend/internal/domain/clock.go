package domain

import "time"

// Clock dá a hora atual. Regras que dependem do "agora" (lançar no dia de hoje, janela de sincronização, próximo resumo) a
// recebem em vez de chamar time.Now(), para os testes poderem fixar o relógio.
type Clock interface {
	Now() time.Time
}

// SystemClock é o relógio real.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

// FixedClock devolve sempre o mesmo instante (testes).
type FixedClock struct{ T time.Time }

func (c FixedClock) Now() time.Time { return c.T }
