import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Bot, CircleHelp, Trash2 } from 'lucide-react'
import { useState, type CSSProperties } from 'react'
import { Link } from 'react-router'
import { deleteCoachAnalysis, setCoachAnswer } from '../api/client'
import type { CoachAnalysis } from '../api/types'
import { cleanLabel } from '../lib/budget'
import { byPriority, priorityLabel, questionsOf, type CoachQuestion } from '../lib/coach'
import { formatBRL } from '../lib/format'
import { KIND_META, savingLine, transactionsLink } from '../lib/review'
import { CategoryChip } from './CategoryChip'

const ANSWER_MAX = 300

/** Uma análise guardada: texto da IA, números do app, perguntas com campo de resposta e apagar. */
export function CoachAnalysisCard({ analysis, month }: { analysis: CoachAnalysis; month: string }) {
  const queryClient = useQueryClient()
  const { advice } = analysis
  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ['coach-analyses'] })
    // As respostas entram na memória do próximo envio, então a prévia (e o hash) mudam.
    queryClient.invalidateQueries({ queryKey: ['coach-preview'] })
  }
  const remove = useMutation({ mutationFn: () => deleteCoachAnalysis(analysis.id), onSuccess: refresh })
  const when = new Date(analysis.createdAt).toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' })

  return (
    <section className="coach-analysis" aria-label={`Análise de ${when}`}>
      <h3 className="bill-head">
        <Bot size={18} aria-hidden="true" /> Análise de {when}
        <button
          type="button"
          className="btn btn-small"
          disabled={remove.isPending}
          onClick={() => window.confirm('Apagar esta análise e as suas respostas?') && remove.mutate()}
        >
          <Trash2 size={13} aria-hidden="true" /> Apagar
        </button>
      </h3>
      {advice.summary && <p className="card coach-summary">{advice.summary}</p>}

      <div className="bill-grid section-gap">
        {byPriority(advice.actions).map((a) => (
          <article key={a.suggestionId} className="bill" style={{ '--c': 'var(--accent)' } as CSSProperties}>
            {a.suggestion && (
              <>
                <div className="bill-top">
                  <CategoryChip category={a.suggestion.category} size={20} />
                  <span className="bill-name">
                    <strong>{cleanLabel(a.suggestion.label)}</strong>
                    <span className="bill-meta">
                      {KIND_META[a.suggestion.kind].label} · {formatBRL(a.suggestion.amount)} no mês
                    </span>
                  </span>
                </div>
                <p className="saving">
                  <span className="saving-label">Economia possível (calculada pelo app)</span>{' '}
                  <strong>{savingLine(a.suggestion.monthly, a.suggestion.annual)}</strong>
                </p>
              </>
            )}
            <p className="goal-status">{priorityLabel(a.priority)}</p>
            {a.comment && <p>{a.comment}</p>}
            {a.question && (
              <>
                <p className="coach-question">
                  <CircleHelp size={14} aria-hidden="true" /> {a.question}
                </p>
                <Answer analysis={analysis} q={{ key: `a:${a.suggestionId}`, text: a.question, about: null }} onSaved={refresh} />
              </>
            )}
            {a.suggestion && (
              <div className="bill-foot">
                <Link className="link" to={transactionsLink(month, a.suggestion.category)}>
                  Ver transações de {a.suggestion.categoryLabel}
                </Link>
                <Link className="link" to={`/revisao?mes=${month}`}>
                  Dispensar na Revisão
                </Link>
              </div>
            )}
          </article>
        ))}
      </div>

      {advice.goals.length > 0 && (
        <>
          <h4 className="coach-sub">Sobre as suas metas</h4>
          <ul className="coach-names">
            {advice.goals.map((g) => (
              <li key={g.goalId}>
                <strong>{g.name}:</strong> {g.comment}
              </li>
            ))}
          </ul>
        </>
      )}
      {advice.questions.length > 0 && (
        <>
          <h4 className="coach-sub">Perguntas para pensar</h4>
          {questionsOf(advice)
            .filter((q) => q.key.startsWith('q:'))
            .map((q) => (
              <div key={q.key} className="coach-general">
                <p className="coach-question">
                  <CircleHelp size={14} aria-hidden="true" /> {q.text}
                </p>
                <Answer analysis={analysis} q={q} onSaved={refresh} />
              </div>
            ))}
        </>
      )}
      {remove.isError && (
        <p className="state error" role="alert">
          Não foi possível apagar. Tente de novo.
        </p>
      )}
    </section>
  )
}

/** Campo de resposta curta a uma pergunta da IA. A resposta salva entra no contexto das próximas análises. */
function Answer({ analysis, q, onSaved }: { analysis: CoachAnalysis; q: CoachQuestion; onSaved: () => void }) {
  const saved = analysis.answers[q.key] ?? ''
  const [text, setText] = useState(saved)
  const save = useMutation({ mutationFn: (answer: string) => setCoachAnswer(analysis.id, q.key, answer), onSuccess: onSaved })
  const dirty = text.trim() !== saved

  return (
    <form
      className="coach-answer"
      onSubmit={(e) => {
        e.preventDefault()
        save.mutate(text.trim())
      }}
    >
      <label>
        <span className="sr-only">Sua resposta: {q.text}</span>
        <textarea
          rows={2}
          maxLength={ANSWER_MAX}
          placeholder="Sua resposta (opcional, até 300 caracteres)"
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
      </label>
      <div className="form-actions">
        <button type="submit" className="btn btn-small" disabled={!dirty || save.isPending}>
          {saved && text.trim() === '' ? 'Apagar resposta' : 'Salvar resposta'}
        </button>
        {saved && !dirty && <span className="tile-note">Resposta salva. A IA vai lembrar dela na próxima análise.</span>}
        {save.isError && (
          <span className="state error" role="alert">
            Não foi possível salvar.
          </span>
        )}
      </div>
    </form>
  )
}
