package httpserver

import (
	"encoding/base64"
	"strings"
)

func maskPhone(phone string) string {
	if idx := strings.Index(phone, "@"); idx != -1 {
		phone = phone[:idx]
	}
	if len(phone) <= 4 {
		return "***"
	}
	return "***" + phone[len(phone)-4:]
}

func decodeBase64Image(data string) ([]byte, error) {
	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}

	return base64.StdEncoding.DecodeString(data)
}
