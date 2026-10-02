package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
)

const (
	maxCoachBody     = 1 << 10
	coachTimeout     = 45 * time.Second
	coachBlockedText = "O Coach envia valores e nomes de serviços ao Google Gemini. No plano gratuito o Google pode usar e revisar esse conteúdo, " +
		"então ele só funciona com o plano pago: ative o faturamento do projeto da chave, defina GEMINI_PAID_PLAN=true no .env e reinicie o app."
)

// coachService guarda o estado do Coach: uma análise por vez (a quantidade de análises é problema de quem hospeda).
type coachService struct {
	advisor ports.Coach
	paid    bool

	busy atomic.Bool
}

func newCoachService(advisor ports.Coach, paid bool) *coachService {
	return &coachService{advisor: advisor, paid: paid}
}

func (c *coachService) enabled() bool { return c.advisor != nil && c.paid }

// begin reserva a análise; recusa se já há uma em andamento (clique duplo, aba aberta em dois lugares).
func (c *coachService) begin() bool { return c.busy.CompareAndSwap(false, true) }

func (c *coachService) end() { c.busy.Store(false) }

// coachInput junta, para o mês, o contexto enviado à IA e o que é preciso para ligar a resposta aos números do código.
type coachInput struct {
	payload    []byte
	hash       string
	context    usecase.CoachContext
	candidates []usecase.Candidate
	goals      []usecase.GoalProgress
}

func (a *api) coachInput(ctx context.Context, month time.Time) (coachInput, error) {
	rev, err := a.buildReview(ctx, month)
	if err != nil {
		return coachInput{}, err
	}
	totals, err := a.reader.MonthlyTotals(ctx, month, month)
	if err != nil || len(totals) != 1 {
		return coachInput{}, errors.Join(err, errors.New("totais do mês indisponíveis"))
	}
	goals, _, err := a.goalProgress(ctx)
	if err != nil {
		return coachInput{}, err
	}
	in := coachInput{context: usecase.BuildCoachContext(rev, formatMonth(month), totals[0], goals), candidates: usecase.CoachCandidates(rev), goals: goals}
	if in.payload, err = json.Marshal(in.context); err != nil {
		return coachInput{}, err
	}
	sum := sha256.Sum256(in.payload)
	in.hash = hex.EncodeToString(sum[:])
	return in, nil
}

// coachPreview mostra exatamente o que seria enviado à IA, sem enviar nada.
func (a *api) coachPreview(w http.ResponseWriter, r *http.Request) {
	month, ok := a.monthParam(w, r, "month", true)
	if !ok {
		return
	}
	in, err := a.coachInput(r.Context(), month)
	if err != nil {
		a.fail(w, "coach", err)
		return
	}
	blocked := ""
	if !a.coach.enabled() {
		blocked = coachBlockedText
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": a.coach.enabled(), "blockedReason": blocked, "provider": "Google Gemini", "month": formatMonth(month),
		"bytes": len(in.payload), "hash": in.hash, "context": json.RawMessage(in.payload),
	})
}

// coachAnalyze envia o contexto à IA (uma chamada) e devolve a resposta validada, ligada aos números do código.
// Só envia os dados que a pessoa viu: o hash da prévia precisa bater com o contexto de agora.
func (a *api) coachAnalyze(w http.ResponseWriter, r *http.Request) {
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" || !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "requisição não permitida")
		return
	}
	var body struct {
		Month string `json:"month"`
		Hash  string `json:"hash"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCoachBody)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	month, err := time.Parse("2006-01", body.Month)
	if err != nil {
		writeError(w, http.StatusBadRequest, "month deve estar no formato AAAA-MM")
		return
	}
	if !a.coach.enabled() {
		writeError(w, http.StatusForbidden, coachBlockedText)
		return
	}
	if !a.coach.begin() {
		writeError(w, http.StatusTooManyRequests, "já há uma análise em andamento")
		return
	}
	defer a.coach.end()

	in, err := a.coachInput(r.Context(), month)
	if err != nil {
		a.fail(w, "coach", err)
		return
	}
	if body.Hash != in.hash {
		writeError(w, http.StatusConflict, "os dados mudaram desde a prévia: confira de novo o que será enviado")
		return
	}
	if len(in.payload) > usecase.MaxCoachBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "contexto grande demais para enviar")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), coachTimeout)
	defer cancel()
	raw, err := a.coach.advisor.Advise(ctx, in.payload)
	if err != nil {
		a.fail(w, "coach", err)
		return
	}
	advice, err := usecase.ValidateAdvice(raw, in.context)
	if errors.Is(err, usecase.ErrEmptyAdvice) {
		writeError(w, http.StatusBadGateway, "a IA não devolveu uma resposta utilizável; tente de novo")
		return
	}
	if err != nil {
		a.fail(w, "coach", err)
		return
	}

	type suggestion struct {
		Kind          string   `json:"kind"`
		Key           string   `json:"key"`
		Label         string   `json:"label"`
		Category      string   `json:"category"`
		CategoryLabel string   `json:"categoryLabel"`
		Monthly       float64  `json:"monthly"`
		Annual        *float64 `json:"annual"`
		Amount        float64  `json:"amount"`
	}
	type action struct {
		SuggestionID string      `json:"suggestionId"`
		Priority     int         `json:"priority"`
		Comment      string      `json:"comment"`
		Question     string      `json:"question"`
		Suggestion   *suggestion `json:"suggestion"`
	}
	type note struct {
		GoalID  string `json:"goalId"`
		Name    string `json:"name"`
		Comment string `json:"comment"`
	}
	actions := make([]action, 0, len(advice.Actions))
	for _, act := range advice.Actions {
		out := action{SuggestionID: act.SuggestionID, Priority: act.Priority, Comment: act.Comment, Question: act.Question}
		// Os números mostrados vêm dos detectores, nunca do texto da IA.
		if i, err := strconv.Atoi(act.SuggestionID[1:]); err == nil && i >= 1 && i <= len(in.candidates) {
			c := in.candidates[i-1]
			s := suggestion{Kind: c.Kind, Key: c.Key, Label: c.Label, Category: c.Category, CategoryLabel: domain.Category(c.Category).Label(), Monthly: c.Saving, Amount: c.Amount}
			if c.Label == "" {
				s.Label = s.CategoryLabel
			}
			if c.Recurring {
				annual := c.Saving * 12
				s.Annual = &annual
			}
			out.Suggestion = &s
		}
		actions = append(actions, out)
	}
	notes := make([]note, 0, len(advice.Goals))
	for _, g := range advice.Goals {
		if i, err := strconv.Atoi(g.GoalID[1:]); err == nil && i >= 1 && i <= len(in.goals) {
			notes = append(notes, note{GoalID: g.GoalID, Name: in.goals[i-1].Goal.Name, Comment: g.Comment})
		}
	}
	questions := advice.Questions
	if questions == nil {
		questions = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"summary": advice.Summary, "actions": actions, "goals": notes, "questions": questions,
	})
}
