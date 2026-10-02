package domain

import "time"

// Dismissal é uma sugestão da revisão que a pessoa dispensou.
type Dismissal struct {
	Kind string
	Key  string
}

// Decision é um "cancelei" sobre uma sugestão recorrente da revisão (kind FIXED ou ANT).
type Decision struct {
	Kind     string
	Key      string
	Label    string
	Category string
	Month    time.Time // mês em que a pessoa decidiu (primeiro dia)
	Monthly  float64   // quanto a conta custava por mês nesse mês
}
