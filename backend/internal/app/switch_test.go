package app

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

type recordingMessenger struct {
	mu   sync.Mutex
	name string
	to   []string
}

func (m *recordingMessenger) SendText(_ context.Context, to, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.to = append(m.to, to)
	return "", nil
}

func (m *recordingMessenger) SendDocument(_ context.Context, to, _ string, _ []byte, _ string) (string, error) {
	return m.SendText(context.Background(), to, "")
}

func TestSwitch_NoChannelUntilPublished(t *testing.T) {
	var s channelSwitch
	if s.up() {
		t.Fatal("sem canal publicado")
	}
	if _, err := s.SendText(context.Background(), "x", "oi"); !errors.Is(err, ports.ErrNoChannel) {
		t.Fatalf("sem canal = %v", err)
	}
	if _, err := s.SendDocument(context.Background(), "x", "a.csv", nil, ""); !errors.Is(err, ports.ErrNoChannel) {
		t.Fatalf("sem canal (documento) = %v", err)
	}
	m := &recordingMessenger{}
	s.publish(m, "dono")
	if _, err := s.SendText(context.Background(), "outra-pessoa", "oi"); err != nil || !s.up() {
		t.Fatalf("com canal = %v", err)
	}
	if len(m.to) != 1 || m.to[0] != "dono" {
		t.Fatalf("o canal trocável só fala com o dono: %v", m.to)
	}
}

// Rode com -race: quem envia enquanto o canal é trocado vê o canal antigo ou o novo, nunca um meio pronto.
func TestSwitch_ConcurrentPublishAndSend(t *testing.T) {
	var s channelSwitch
	var wg sync.WaitGroup
	msgs := []*recordingMessenger{{name: "a"}, {name: "b"}}
	for i := range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.publish(msgs[i%2], "dono-"+msgs[i%2].name)
		}()
		go func() {
			defer wg.Done()
			for range 50 {
				if _, err := s.SendText(context.Background(), "", "oi"); err != nil && !errors.Is(err, ports.ErrNoChannel) {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	for _, m := range msgs {
		for _, to := range m.to {
			if to != "dono-"+m.name {
				t.Fatalf("mensagem de %s foi para %s", m.name, to)
			}
		}
	}
}
