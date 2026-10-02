import './styles.css'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Bot, ShieldAlert } from 'lucide-react'
import { useState } from 'react'
import { CoachAnalysisCard } from './components/CoachAnalysisCard'
import { MonthFilter } from '../../shared/components/MonthFilter'
import { QueryState } from '../../shared/components/QueryState'
import { describeContext, sentNames } from './lib/coach'
import { formatMonthTitle } from '../../shared/lib/format'
import { useMonth } from '../../shared/lib/useMonth'
import { analyzeCoach, useCoachAnalyses, useCoachPreview } from './api'
import { ApiError } from '../../shared/api/request'
import type { CoachPreview } from './api'

export default function Coach() {
  const { month, setMonth, now } = useMonth()
  const preview = useCoachPreview(month)
  const analyses = useCoachAnalyses(month)
  const queryClient = useQueryClient()
  // O consentimento vale para os dados exatos que a pessoa viu (o hash); mudou o mês ou os dados, vale de novo conferir.
  const [agreedHash, setAgreedHash] = useState<string | null>(null)

  const analyze = useMutation({
    mutationFn: ({ hash }: { hash: string }) => analyzeCoach(month, hash),
    // Depois de cada tentativa relê a prévia: se os dados mudaram (409), a pessoa confere o novo envio antes de tentar de novo.
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['coach-preview'] })
      queryClient.invalidateQueries({ queryKey: ['coach-analyses'] })
    },
  })
  return (
    <>
      <h1 className="page-title">Coach de {formatMonthTitle(month)}</h1>
      <MonthFilter month={month} now={now} onChange={setMonth} warnPartial />

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

              {analyses.data && analyses.data.length > 0 && (
                <section className="section-gap" aria-labelledby="history-title">
                  <h2 className="chart-title" id="history-title">
                    Análises guardadas ({analyses.data.length})
                  </h2>
                  {analyses.data.map((an, i) =>
                    i === 0 ? (
                      <CoachAnalysisCard key={an.id} analysis={an} month={month} />
                    ) : (
                      <details key={an.id} className="coach-older">
                        <summary>
                          Análise anterior de {new Date(an.createdAt).toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' })}
                        </summary>
                        <CoachAnalysisCard analysis={an} month={month} />
                      </details>
                    ),
                  )}
                </section>
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
