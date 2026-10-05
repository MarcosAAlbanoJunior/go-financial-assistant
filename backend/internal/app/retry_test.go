package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryBackoff_GrowsUntilSuccess(t *testing.T) {
	var waits []time.Duration
	calls := 0
	ok := retryBackoff(context.Background(), time.Millisecond, 4*time.Millisecond, func() error {
		calls++
		if calls < 4 {
			return errors.New("fora do ar")
		}
		return nil
	}, func(_ error, next time.Duration) { waits = append(waits, next) })
	if !ok || calls != 4 {
		t.Fatalf("ok=%v calls=%d", ok, calls)
	}
	want := []time.Duration{2 * time.Millisecond, 4 * time.Millisecond, 4 * time.Millisecond}
	if len(waits) != len(want) {
		t.Fatalf("esperas %v", waits)
	}
	for i := range want {
		if waits[i] != want[i] {
			t.Fatalf("esperas %v, queria %v (dobra até o teto)", waits, want)
		}
	}
}

func TestRetryBackoff_StopsWhenAppStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	done := make(chan bool)
	go func() {
		done <- retryBackoff(ctx, time.Millisecond, time.Millisecond, func() error {
			calls++
			if calls == 3 {
				cancel()
			}
			return errors.New("fora do ar")
		}, func(error, time.Duration) {})
	}()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("encerrado não é sucesso")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("não parou com o app")
	}
}
