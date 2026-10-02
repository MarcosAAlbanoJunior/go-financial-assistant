package domain

import (
	"time"

	"github.com/google/uuid"
)

// CoachAnalysis é uma análise guardada. Advice é o JSON da análise (formato definido pelo usecase) e Answers,
// as respostas da pessoa às perguntas, por chave.
type CoachAnalysis struct {
	ID        uuid.UUID
	Month     time.Time
	CreatedAt time.Time
	Advice    []byte
	Answers   map[string]string
}
