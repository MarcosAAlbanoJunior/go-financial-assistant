package domain

import (
	"time"

	"github.com/google/uuid"
)

type GoalKind string

const (
	GoalSave    GoalKind = "SAVE"
	GoalCut     GoalKind = "CUT"
	GoalReserve GoalKind = "RESERVE"
)

// Goal é uma meta financeira; cada tipo usa só os seus campos.
type Goal struct {
	ID            uuid.UUID
	Kind          GoalKind
	Name          string
	TargetAmount  float64   // SAVE
	TargetDate    time.Time // SAVE: primeiro dia do mês-alvo
	Category      string    // CUT
	CutPercent    int       // CUT
	Baseline      float64   // CUT: média mensal da categoria antes da meta
	ReserveMonths int       // RESERVE
	CreatedAt     time.Time
}
