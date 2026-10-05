package app

import (
	"context"
	"sync/atomic"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// liveChannel é um canal de conversa no ar: por onde mandar e quem é o dono.
type liveChannel struct {
	messenger ports.Messenger
	owner     string
}

// channelSwitch é o canal de conversa que pode ligar com o app rodando (setup concluído, Telegram que voltou). Quem usa o
// chat (resumo, relatório, avisos, código do segundo fator) recebe ele no boot e não precisa saber quando o canal subiu.
// O canal é montado por inteiro antes de publicado: quem lê vê o canal antigo ou o novo, nunca um meio pronto.
type channelSwitch struct {
	live atomic.Pointer[liveChannel]
}

func (s *channelSwitch) publish(messenger ports.Messenger, owner string) {
	s.live.Store(&liveChannel{messenger: messenger, owner: owner})
}

// up diz se há canal no ar.
func (s *channelSwitch) up() bool { return s.live.Load() != nil }

// SendText manda ao dono do canal no ar. O destinatário pedido é ignorado: o canal trocável só fala com o dono.
func (s *channelSwitch) SendText(ctx context.Context, _ string, text string) (string, error) {
	live := s.live.Load()
	if live == nil {
		return "", ports.ErrNoChannel
	}
	return live.messenger.SendText(ctx, live.owner, text)
}

// SendDocument manda o arquivo ao dono do canal no ar (o destinatário pedido é ignorado, como em SendText).
func (s *channelSwitch) SendDocument(ctx context.Context, _ string, filename string, data []byte, caption string) (string, error) {
	live := s.live.Load()
	if live == nil {
		return "", ports.ErrNoChannel
	}
	return live.messenger.SendDocument(ctx, live.owner, filename, data, caption)
}
