package telegram

import (
	"regexp"
	"strings"
)

const (
	maxTextRunes    = 4000 // limite da API: 4096 caracteres por mensagem
	maxCaptionRunes = 1000 // limite da API: 1024 caracteres por legenda
)

var (
	boldPattern = regexp.MustCompile(`\*([^*\n]+)\*`)
	htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
)

// toHTML converte o texto padrão do bot (negrito no estilo WhatsApp, *assim*) para o
// parse_mode HTML do Telegram, escapando antes qualquer <, > e & vindos de descrições.
func toHTML(s string, maxRunes int) string {
	if r := []rune(s); len(r) > maxRunes {
		s = string(r[:maxRunes]) + "…"
	}
	return boldPattern.ReplaceAllString(htmlEscaper.Replace(s), "<b>$1</b>")
}
