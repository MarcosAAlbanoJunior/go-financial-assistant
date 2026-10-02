package format

import "testing"

func TestFormatBRL(t *testing.T) {
	for in, want := range map[float64]string{
		0: "R$ 0,00", 5: "R$ 5,00", 1433.6: "R$ 1.433,60", 1234567.891: "R$ 1.234.567,89", 0.005: "R$ 0,01", -19.9: "-R$ 19,90", -0.001: "R$ 0,00",
	} {
		if got := FormatBRL(in); got != want {
			t.Errorf("FormatBRL(%v) = %q, esperava %q", in, got, want)
		}
	}
}
