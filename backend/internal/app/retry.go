package app

import (
	"context"
	"time"
)

// retryBackoff espera e tenta de novo, com espera crescente (começa em first e dobra a cada falha, até max), até attempt
// dar certo ou o contexto acabar. Devolve false só se o contexto acabou antes.
func retryBackoff(ctx context.Context, first, max time.Duration, attempt func() error, failed func(err error, next time.Duration)) bool {
	for delay := first; ; delay = min(delay*2, max) {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(delay):
		}
		if err := attempt(); err != nil {
			if ctx.Err() != nil {
				return false
			}
			failed(err, min(delay*2, max))
			continue
		}
		return true
	}
}
