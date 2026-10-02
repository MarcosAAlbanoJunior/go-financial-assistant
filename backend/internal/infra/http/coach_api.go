package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/planning"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase"
	"github.com/google/uuid"
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
	paidFn  func() bool // quando definido, vale no lugar de paid (configuração editável no dashboard)

	busy atomic.Bool
}

func newCoachService(advisor ports.Coach, paid bool) *coachService {
	return &coachService{advisor: advisor, paid: paid}
}

func (c *coachService) enabled() bool {
	paid := c.paid
	if c.paidFn != nil {
		paid = c.paidFn()
	}
	return c.advisor != nil && paid
}

// begin reserva a análise; recusa se já há uma em andamento (clique duplo, aba aberta em dois lugares).
func (c *coachService) begin() bool { return c.busy.CompareAndSwap(false, true) }

func (c *coachService) end() { c.busy.Store(false) }

// coachInput junta, para o mês, o contexto enviado à IA e o que é preciso para ligar a resposta aos números do código.
type coachInput struct {
	payload    []byte
	hash       string
	context    usecase.CoachContext
	candidates []usecase.Candidate
	goals      []planning.GoalProgress
}

func (a *api) coachInput(ctx context.Context, month time.Time) (coachInput, error) {
	rev, err := a.insights.Review(ctx, month)
	if err != nil {
		return coachInput{}, err
	}
	totals, err := a.reader.MonthlyTotals(ctx, month, month)
	if err != nil || len(totals) != 1 {
		return coachInput{}, errors.Join(err, errors.New("totais do mês indisponíveis"))
	}
	goals, _, err := a.insights.Goals(ctx, a.now())
	if err != nil {
		return coachInput{}, err
	}
	past, err := a.reader.CoachAnalyses(ctx, nil, usecase.MaxCoachMemory)
	if err != nil {
		return coachInput{}, err
	}
	savings, err := a.insights.Savings(ctx, a.monthStart())
	if err != nil {
		return coachInput{}, err
	}
	in := coachInput{context: usecase.BuildCoachContext(rev, formatMonth(month), totals[0], goals, past, savings), candidates: usecase.CoachCandidates(rev), goals: goals}
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
	var body struct {
		Month string `json:"month"`
		Hash  string `json:"hash"`
	}
	if !decodeJSON(w, r, maxCoachBody, &body) {
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

	analysis, err := a.runCoachAnalysis(r.Context(), month, body.Hash)
	if err != nil {
		a.respondErr(w, "coach", err)
		return
	}
	writeJSON(w, http.StatusOK, analysisJSON(analysis))
}

// runCoachAnalysis monta o contexto, confere que é o que a pessoa viu na prévia (hash), pede a análise à IA, valida a
// resposta e a grava. Erros de regra voltam como apiError; o resto é falha interna.
func (a *api) runCoachAnalysis(ctx context.Context, month time.Time, hash string) (domain.CoachAnalysis, error) {
	in, err := a.coachInput(ctx, month)
	if err != nil {
		return domain.CoachAnalysis{}, err
	}
	if hash != in.hash {
		return domain.CoachAnalysis{}, &apiError{http.StatusConflict, "os dados mudaram desde a prévia: confira de novo o que será enviado"}
	}
	if len(in.payload) > usecase.MaxCoachBytes {
		return domain.CoachAnalysis{}, &apiError{http.StatusRequestEntityTooLarge, "contexto grande demais para enviar"}
	}

	adviceCtx, cancel := context.WithTimeout(ctx, coachTimeout)
	defer cancel()
	raw, err := a.coach.advisor.Advise(adviceCtx, in.payload)
	if err != nil {
		return domain.CoachAnalysis{}, err
	}
	advice, err := usecase.ValidateAdvice(raw, in.context)
	if errors.Is(err, usecase.ErrEmptyAdvice) {
		return domain.CoachAnalysis{}, &apiError{http.StatusBadGateway, "a IA não devolveu uma resposta utilizável; tente de novo"}
	}
	if err != nil {
		return domain.CoachAnalysis{}, err
	}

	record, err := json.Marshal(usecase.NewCoachRecord(advice, in.candidates, in.goals))
	if err != nil {
		return domain.CoachAnalysis{}, err
	}
	analysis := domain.CoachAnalysis{ID: uuid.New(), Month: month, Advice: record, Answers: map[string]string{}}
	if err := a.reader.SaveCoachAnalysis(ctx, analysis); err != nil {
		return domain.CoachAnalysis{}, err
	}
	analysis.CreatedAt = a.now()
	return analysis, nil
}

func analysisJSON(an domain.CoachAnalysis) map[string]any {
	answers := an.Answers
	if answers == nil {
		answers = map[string]string{}
	}
	return map[string]any{"id": an.ID, "month": formatMonth(an.Month), "createdAt": an.CreatedAt, "advice": json.RawMessage(an.Advice), "answers": answers}
}

const maxCoachHistory = 20

// coachAnalyses lista as análises guardadas do mês, da mais nova para a mais antiga.
func (a *api) coachAnalyses(w http.ResponseWriter, r *http.Request) {
	month, ok := a.monthParam(w, r, "month", true)
	if !ok {
		return
	}
	list, err := a.reader.CoachAnalyses(r.Context(), &month, maxCoachHistory)
	if err != nil {
		a.fail(w, "coach", err)
		return
	}
	out := make([]map[string]any, len(list))
	for i, an := range list {
		out[i] = analysisJSON(an)
	}
	writeJSON(w, http.StatusOK, out)
}

// coachAnswerKeyRE: "a:s3" (pergunta sobre a sugestão s3) ou "q:0" (pergunta geral).
var coachAnswerKeyRE = regexp.MustCompile(`^(a:s\d{1,2}|q:\d)$`)

// setCoachAnswer guarda (ou apaga, se vazia) a resposta da pessoa a uma pergunta da IA.
func (a *api) setCoachAnswer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	var body struct {
		Key    string `json:"key"`
		Answer string `json:"answer"`
	}
	if !decodeJSON(w, r, maxCoachBody, &body) {
		return
	}
	if !coachAnswerKeyRE.MatchString(body.Key) {
		writeError(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	answer := ""
	if strings.TrimSpace(body.Answer) != "" { // vazio apaga a resposta
		var ok bool
		if answer, ok = usecase.CleanAnswer(body.Answer); !ok {
			writeError(w, http.StatusBadRequest, "a resposta deve ter até 300 caracteres, sem caracteres de controle")
			return
		}
	}
	found, err := a.reader.SetCoachAnswer(r.Context(), id, body.Key, answer)
	if err != nil {
		a.fail(w, "coach", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "análise não encontrada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *api) deleteCoachAnalysis(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	found, err := a.reader.DeleteCoachAnalysis(r.Context(), id)
	if err != nil {
		a.fail(w, "coach", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "análise não encontrada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *api) registerCoach(rt routes) {
	rt.mux.Handle("GET /api/coach/preview", rt.protected(a.coachPreview))
	rt.mux.Handle("GET /api/coach/analyses", rt.protected(a.coachAnalyses))
	rt.mux.Handle("PUT /api/coach/analyses/{id}/answers", rt.protected(jsonOnly(a.setCoachAnswer)))
	rt.mux.Handle("DELETE /api/coach/analyses/{id}", rt.protected(sameOriginOnly(a.deleteCoachAnalysis)))
	// A análise custa dinheiro e sai da máquina: limite apertado por IP (contra clique repetido ou loop).
	rt.mux.Handle("POST /api/coach/analyze", newIPRateLimiter(3, time.Minute).middleware(rt.protected(jsonOnly(a.coachAnalyze))))
}
