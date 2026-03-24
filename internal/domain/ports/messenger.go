package ports

import "context"

type Messenger interface {
	SendText(ctx context.Context, to string, text string) (messageID string, err error)
	FetchImageBase64(ctx context.Context, remoteJid string, fromMe bool, messageID string) (string, error)
}
