package ports

import "context"

// Messenger envia mensagens ao usuário em qualquer canal (WhatsApp, Telegram).
// O messageID retornado serve apenas para canais que precisam reconhecer o eco
// das próprias mensagens (WhatsApp); os demais podem retornar "".
type Messenger interface {
	SendText(ctx context.Context, to string, text string) (messageID string, err error)
	SendDocument(ctx context.Context, to, filename string, data []byte, caption string) (messageID string, err error)
}
