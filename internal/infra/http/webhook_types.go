package httpserver

import "encoding/json"

type evolutionEnvelope struct {
	Event    string          `json:"event"`
	Instance string          `json:"instance"`
	Data     json.RawMessage `json:"data"`
}

type evolutionPayload struct {
	Instance string        `json:"instance"`
	Data     evolutionData `json:"data"`
}

type evolutionData struct {
	Message evolutionMessage `json:"message"`
	Key     evolutionKey     `json:"key"`
	Base64  string           `json:"base64"`
}

type evolutionKey struct {
	RemoteJID string `json:"remoteJid"`
	FromMe    bool   `json:"fromMe"`
	ID        string `json:"id"`
}

type evolutionMessage struct {
	Conversation        string                 `json:"conversation,omitempty"`
	ImageMessage        *evolutionImageMessage `json:"imageMessage,omitempty"`
	ExtendedTextMessage *evolutionExtendedText `json:"extendedTextMessage,omitempty"`
}

type evolutionImageMessage struct {
	Mimetype string `json:"mimetype"`
	Caption  string `json:"caption"`
	URL      string `json:"url"`
}

type evolutionExtendedText struct {
	Text string `json:"text"`
}
