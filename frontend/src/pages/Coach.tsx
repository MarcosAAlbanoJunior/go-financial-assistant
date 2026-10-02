import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Bot, CircleHelp, ShieldAlert } from 'lucide-react'
import { useState, type CSSProperties } from 'react'
import { Link } from 'react-router'
import { ApiError, analyzeCoach, useCoachPreview } from '../api/client'
import type { CoachAction, CoachPreview, CoachResult } from '../api/types'
import { CategoryChip } from '../components/CategoryChip'
import { MonthFilter } from '../components/MonthFilter'
import { QueryState } from '../components/QueryState'
import { byPriority, describeContext, priorityLabel, sentNames } from '../lib/coach'
import { formatBRL, formatMonthTitle } from '../lib/format'
import { cleanLabel } from '../lib/budget'
import { KIND_META, savingLine, transactionsLink } from '../lib/review'
import { useMonth } from '../lib/useMonth'

export default function Coach() {
  const { month, setMonth, now } = useMonth()
  const preview = useCoachPreview(month)
  const queryClient = useQueryClient()
  // O consentimento vale para os dados exatos que a pessoa viu (o hash); mudou o mês ou os dados, vale de novo conferir.
  const [agreedHash, setAgreedHash] = useState<string | null>(null)

  const analyze = useMutation({
    mutationFn: ({ hash }: { hash: string }) => analyzeCoach(month, hash),
    // Depois de cada tentativa relê a prévia: se os dados mudaram (409), a pessoa confere o novo envio antes de tentar de novo.
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['coach-preview'] }),
  })
  return (
    <>
      <h1 className="page-title">Coach de {formatMonthTitle(month)}</h1>
      <MonthFilter month={month} now={now} onChange={setMonth} />

      <QueryState query={preview}>
        {(p) => {
          const agreed = agreedHash === p.hash
          return (
            <>
              <SendPreview p={p} />
              {p.enabled ? (
                <section className="card section-gap" aria-labelledby="send-title">
                  <h2 className="chart-title" id="send-title">
                    Enviar para análise
                  </h2>
                  <label className="check">
                    <input type="checkbox" checked={agreed} onChange={(e) => setAgreedHash(e.target.checked ? p.hash : null)} />
                    Entendo que os dados acima serão enviados ao {p.provider}.
                  </label>
                  <div className="form-actions section-gap">
                    <button
                      type="button"
                      className="btn btn-primary"
                      disabled={!agreed || analyze.isPending}
                      onClick={() => analyze.mutate({ hash: p.hash })}
                    >
                      <Bot size={14} aria-hidden="true" /> {analyze.isPending ? 'Analisando…' : 'Analisar com IA'}
                    </button>
                  </div>
                  {analyze.isError && (
                    <p className="state error" role="alert">
                      {analyze.error instanceof ApiError ? analyze.error.message : 'Não foi possível analisar. Tente de novo.'}
                    </p>
                  )}
                </section>
              ) : (
                <p className="state error" role="alert">
                  <ShieldAlert size={16} aria-hidden="true" /> {p.blockedReason}
                </p>
              )}

              {analyze.data && analyze.variables?.hash === p.hash && (
                <Result month={month} actions={byPriority(analyze.data.actions)} data={analyze.data} />
              )}

              <p className="tile-note section-gap">
                O Coach só sugere e pergunta: nada muda sozinho. Os valores vêm dos cálculos do app, a IA escreve só o texto e pode errar.
                Isto não é aconselhamento financeiro.
              </p>
            </>
          )
        }}
      </QueryState>
    </>
  )
}

function SendPreview({ p }: { p: CoachPreview }) {
  return (
    <section className="card" aria-labelledby="preview-title">
      <h2 className="chart-title" id="preview-title">
        O que será enviado ao {p.provider}
      </h2>
      <p className="chart-sub">
        Nada é enviado até você clicar em &quot;Analisar com IA&quot;, e nunca em segundo plano. Para tirar um item do envio, dispense-o na
        Revisão.
      </p>
      <ul className="coach-list">
        {describeContext(p.context).map((line) => (
          <li key={line}>{line}</li>
        ))}
      </ul>
      {sentNames(p.context).length > 0 && (
        <>
          <h3 className="coach-sub">Nomes que vão no envio</h3>
          <ul className="coach-names">
            {sentNames(p.context).map((n, i) => (
              <li key={n + i}>{n}</li>
            ))}
          </ul>
        </>
      )}
      <details className="coach-json">
        <summary>Ver o JSON exato ({p.bytes} bytes)</summary>
        <pre>{JSON.stringify(p.context, null, 2)}</pre>
      </details>
    </section>
  )
}

function Result({ month, actions, data }: { month: string; actions: CoachAction[]; data: CoachResult }) {
  return (
    <section className="section-gap" aria-labelledby="result-title">
      <h2 className="bill-head" id="result-title">
        <Bot size={18} aria-hidden="true" /> Análise da IA
      </h2>
      {data.summary && <p className="card coach-summary">{data.summary}</p>}

      <div className="bill-grid section-gap">
        {actions.map((a) => (
          <article key={a.suggestionId} className="bill" style={{ '--c': 'var(--accent)' } as CSSProperties}>
            {a.suggestion ? (
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
            ) : null}
            <p className="goal-status">{priorityLabel(a.priority)}</p>
            {a.comment && <p>{a.comment}</p>}
            {a.question && (
              <p className="coach-question">
                <CircleHelp size={14} aria-hidden="true" /> {a.question}
              </p>
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

      {data.goals.length > 0 && (
        <>
          <h3 className="coach-sub">Sobre as suas metas</h3>
          <ul className="coach-names">
            {data.goals.map((g) => (
              <li key={g.goalId}>
                <strong>{g.name}:</strong> {g.comment}
              </li>
            ))}
          </ul>
        </>
      )}
      {data.questions.length > 0 && (
        <>
          <h3 className="coach-sub">Perguntas para pensar</h3>
          <ul className="coach-names">
            {data.questions.map((q) => (
              <li key={q}>{q}</li>
            ))}
          </ul>
        </>
      )}
    </section>
  )
}
