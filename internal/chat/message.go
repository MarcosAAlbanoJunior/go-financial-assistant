// Package chat contém a lógica de conversa independente de canal: recebe uma
// Message já normalizada pelo adapter (WhatsApp ou Telegram) e responde via ports.Messenger.
package chat

import "context"

type Message struct {
	Text     string
	Image    *Attachment
	Document *Attachment
}

// Attachment carrega o conteúdo sob demanda, pois cada canal obtém a mídia de um jeito.
type Attachment struct {
	MimeType string
	Caption  string
	Load     func(ctx context.Context) ([]byte, error)
}
