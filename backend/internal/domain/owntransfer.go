package domain

import (
	"strings"
	"sync"
)

var accentFolder = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "é", "e", "è", "e", "ê", "e", "í", "i", "ì", "i", "î", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o", "ú", "u", "ù", "u", "û", "u", "ü", "u", "ç", "c",
)

// Fold deixa o texto em minúsculas, sem acento e com espaços compactados, para comparar nomes.
func Fold(s string) string {
	return strings.Join(strings.Fields(accentFolder.Replace(strings.ToLower(s))), " ")
}

var transferWords = []string{"pix", "transfer", "ted ", "doc "}

// OwnTransferMatcher reconhece Pix, TED e DOC com o nome da própria pessoa: mover dinheiro entre contas suas
// não é gasto nem renda. Os nomes podem mudar com o app rodando (página de configurações).
type OwnTransferMatcher struct {
	mu    sync.RWMutex
	names []string // normalizados com Fold
}

func NewOwnTransferMatcher(names []string) *OwnTransferMatcher {
	m := &OwnTransferMatcher{}
	m.Set(names)
	return m
}

func (m *OwnTransferMatcher) Set(names []string) {
	folded := make([]string, 0, len(names))
	for _, n := range names {
		if f := Fold(n); f != "" {
			folded = append(folded, f)
		}
	}
	m.mu.Lock()
	m.names = folded
	m.mu.Unlock()
}

// Match diz se a descrição é uma transferência (Pix, TED, DOC) em que aparece um dos nomes.
func (m *OwnTransferMatcher) Match(description string) bool {
	m.mu.RLock()
	names := m.names
	m.mu.RUnlock()
	if len(names) == 0 {
		return false
	}
	d := Fold(description) + " "
	if !containsAnyOf(d, transferWords) {
		return false
	}
	return containsAnyOf(d, names)
}

func containsAnyOf(s string, parts []string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
