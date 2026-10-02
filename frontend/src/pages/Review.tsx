import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Eye, EyeOff } from 'lucide-react'
import { useState, type CSSProperties } from 'react'
import { Link } from 'react-router'
import { setDecision, setDismissal, useReview } from '../api/client'
import type { ReviewCandidate, ReviewKind } from '../api/types'
import { CategoryChip } from '../components/CategoryChip'
import { HeatMatrix } from '../components/HeatMatrix'
import { MonthFilter } from '../components/MonthFilter'
import { QueryState } from '../components/QueryState'
import { SavingsPanel } from '../components/SavingsPanel'
import { categoryVisual } from '../lib/categoryVisual'
import { formatMonthTitle } from '../lib/format'
import { candidateDetail, candidateName, countByKind, KIND_META, KIND_ORDER, savingText, transactionsLink, visibleCandidates } from '../lib/review'
import { canDecide } from '../lib/savings'
import { useMonth } from '../lib/useMonth'

export default function Review() {
  const { month, setMonth, now } = useMonth()
  const review = useReview(month)
  const queryClient = useQueryClient()
  const [kind, setKind] = useState<ReviewKind | 'ALL'>('ALL')
  const [showDismissed, setShowDismissed] = useState(false)

  const dismiss = useMutation({
    mutationFn: ({ c, dismissed }: { c: ReviewCandidate; dismissed: boolean }) => setDismissal(c.kind, c.key, dismissed),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['review'] }),
  })

  // "Cancelei" também dispensa a sugestão e passa a contar na economia realizada.
  const decide = useMutation({
    mutationFn: (c: ReviewCandidate) => setDecision(c.kind, c.key, month, true),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['review'] })
      queryClient.invalidateQueries({ queryKey: ['savings'] })
      queryClient.invalidateQueries({ queryKey: ['coach-preview'] })
    },
  })

  return (
    <>
      <h1 className="page-title">Revisão de {formatMonthTitle(month)}</h1>
      <MonthFilter month={month} now={now} onChange={setMonth} />

      <SavingsPanel />

      <QueryState query={review}>
        {(r) => {
          const counts = countByKind(r.candidates)
          const list = visibleCandidates(r.candidates, kind, showDismissed)
          const dismissedTotal = r.candidates.filter((c) => c.dismissed).length
          return (
            <>
              <section className="card" aria-labelledby="heat-title">
                <h2 className="chart-title" id="heat-title">
                  Categoria por mês
                </h2>
                <p className="chart-sub">Quanto mais escura a célula, mais a categoria gastou naquele mês, comparado aos outros meses dela.</p>
                <HeatMatrix months={r.months} rows={r.matrix} stale={review.isPlaceholderData} />
              </section>

              <section className="section-gap" aria-labelledby="cand-title">
                <h2 className="bill-head" id="cand-title">
                  {showDismissed ? 'Sugestões dispensadas' : 'O que dá para cortar'} <span className="count">{list.length}</span>
                </h2>
                <div className="filters" role="group" aria-label="Tipo de sugestão">
                  <button type="button" className="btn" aria-pressed={kind === 'ALL'} onClick={() => setKind('ALL')}>
                    Todas
                  </button>
                  {KIND_ORDER.map((k) => {
                    const { label, Icon } = KIND_META[k]
                    return (
                      <button key={k} type="button" className="btn" aria-pressed={kind === k} onClick={() => setKind(k)}>
                        <Icon size={14} aria-hidden="true" /> {label} ({counts[k]})
                      </button>
                    )
                  })}
                </div>

                {(dismiss.isError || decide.isError) && (
                  <p className="state error" role="alert">
                    Não foi possível salvar. Tente de novo.
                  </p>
                )}
                {list.length === 0 ? (
                  <p className="state">{showDismissed ? 'Nenhuma sugestão dispensada.' : 'Nenhuma sugestão desse tipo neste mês.'}</p>
                ) : (
                  <div className="bill-grid">
                    {list.map((c) => (
                      <CandidateCard key={c.kind + c.key} c={c} month={month} busy={dismiss.isPending || decide.isPending} onDismiss={(dismissed) => dismiss.mutate({ c, dismissed })} onDecide={() => decide.mutate(c)} />
                    ))}
                  </div>
                )}
                {(dismissedTotal > 0 || showDismissed) && (
                  <button type="button" className="btn section-gap" onClick={() => setShowDismissed(!showDismissed)}>
                    {showDismissed ? <Eye size={14} aria-hidden="true" /> : <EyeOff size={14} aria-hidden="true" />}{' '}
                    {showDismissed ? 'Voltar às sugestões' : `Ver dispensadas (${dismissedTotal})`}
                  </button>
                )}
              </section>

              <p className="tile-note section-gap">
                Tudo aqui é calculado a partir dos seus lançamentos, sem IA. A economia é uma estimativa: aumento = voltar à média; fixa = cancelar; gasto
                formiga (4 ou mais compras de até R$ 40, fora o Pix) = cortar pela metade; duplicata e conta nova valem uma vez só. Os tipos se sobrepõem,
                então não some as sugestões. Com pouco histórico, aumentos e contas novas ficam menos confiáveis.
              </p>
            </>
          )
        }}
      </QueryState>
    </>
  )
}

function CandidateCard({
  c,
  month,
  busy,
  onDismiss,
  onDecide,
}: {
  c: ReviewCandidate
  month: string
  busy: boolean
  onDismiss: (dismissed: boolean) => void
  onDecide: () => void
}) {
  const { label, Icon } = KIND_META[c.kind]
  return (
    <article className="bill" style={{ '--c': categoryVisual(c.category).color, '--cat': categoryVisual(c.category).color } as CSSProperties}>
      <div className="bill-top">
        <CategoryChip category={c.category} size={20} />
        <span className="bill-name">
          <strong>{candidateName(c)}</strong>
          <span className="bill-meta">{candidateDetail(c)}</span>
        </span>
      </div>
      <p className="saving">
        <span className="saving-label">Economia possível</span> <strong>{savingText(c)}</strong>
      </p>
      <div className="bill-foot">
        <span className="status">
          <Icon size={12} aria-hidden="true" /> {label}
        </span>
        <Link className="link" to={transactionsLink(month, c.category)}>
          Ver transações de {c.categoryLabel}
        </Link>
        {canDecide(c.kind) && !c.dismissed && (
          <button type="button" className="btn btn-small" disabled={busy} onClick={onDecide}>
            Cancelei
          </button>
        )}
        <button type="button" className="btn btn-small" disabled={busy} onClick={() => onDismiss(!c.dismissed)}>
          {c.dismissed ? 'Voltar a sugerir' : 'Dispensar'}
        </button>
      </div>
    </article>
  )
}
