package httpserver

import (
	"encoding/base64"
	"strings"
)

func decodeBase64Image(data string) ([]byte, error) {
	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}

	return base64.StdEncoding.DecodeString(data)
}
