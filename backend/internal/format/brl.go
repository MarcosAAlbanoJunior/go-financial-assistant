// Package format reúne formatações de texto compartilhadas.
package format

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// FormatBRL escreve um valor em reais no padrão brasileiro: R$ 1.234,56.
func FormatBRL(v float64) string {
	cents := int64(math.Round(math.Abs(v) * 100))
	digits := strconv.FormatInt(cents/100, 10)
	var groups []string
	for len(digits) > 3 {
		groups = append([]string{digits[len(digits)-3:]}, groups...)
		digits = digits[:len(digits)-3]
	}
	groups = append([]string{digits}, groups...)
	out := fmt.Sprintf("R$ %s,%02d", strings.Join(groups, "."), cents%100)
	if v < 0 && cents > 0 {
		out = "-" + out
	}
	return out
}
