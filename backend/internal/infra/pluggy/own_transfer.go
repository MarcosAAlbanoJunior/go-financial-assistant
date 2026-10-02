package pluggy

import "strings"

var accentFolder = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "é", "e", "è", "e", "ê", "e", "í", "i", "ì", "i", "î", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o", "ú", "u", "ù", "u", "û", "u", "ü", "u", "ç", "c",
)

// fold deixa o texto em minúsculas, sem acento e com espaços compactados, para comparar nomes.
func fold(s string) string {
	return strings.Join(strings.Fields(accentFolder.Replace(strings.ToLower(s))), " ")
}

// SetOwnNames informa os seus nomes (como aparecem nas descrições do banco). Pix e transferências em que
// um deles aparece são entre contas suas: mover dinheiro de lugar não é gasto nem renda.
func (c *Client) SetOwnNames(names []string) {
	c.ownNames = c.ownNames[:0]
	for _, n := range names {
		if f := fold(n); f != "" {
			c.ownNames = append(c.ownNames, f)
		}
	}
}

var transferWords = []string{"pix", "transfer", "ted ", "doc "}

// isOwnTransfer diz se a descrição é uma transferência (Pix, TED, DOC) com o seu próprio nome.
func (c *Client) isOwnTransfer(description string) bool {
	if len(c.ownNames) == 0 {
		return false
	}
	d := fold(description) + " "
	if !containsAny(d, transferWords) {
		return false
	}
	return containsAny(d, c.ownNames)
}
