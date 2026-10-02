import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Check, CircleAlert, Plus, Trash2, TriangleAlert } from 'lucide-react'
import { useState, type CSSProperties } from 'react'
import { ApiError, createGoal, deleteGoal, useGoals } from '../api/client'
import type { Goal } from '../api/types'
import { GoalForm } from '../components/GoalForm'
import { QueryState } from '../components/QueryState'
import { StatTile } from '../components/StatTile'
import { formatBRL, formatMonthShort, formatPercent } from '../lib/format'
import { GOAL_KINDS, goalStatus, progressRatio, type Tone } from '../lib/goals'

const TONE_ICON = { ok: Check, warning: TriangleAlert, critical: CircleAlert }

export default function Goals() {
  const goals = useGoals()
  const queryClient = useQueryClient()
  const [adding, setAdding] = useState(false)
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['goals'] })

  const create = useMutation({
    mutationFn: createGoal,
    onSuccess: () => {
      setAdding(false)
      return refresh()
    },
  })
  const remove = useMutation({ mutationFn: deleteGoal, onSuccess: refresh })

  return (
    <>
      <h1 className="page-title">Metas</h1>
      <QueryState query={goals}>
        {(data) => (
          <>
            <div className="tiles">
              <StatTile
                label="Patrimônio hoje"
                value={formatBRL(data.wealth)}
                note="Contas correntes + investimentos. É o que mede as metas de juntar e de reserva."
              />
            </div>

            {remove.isError && (
              <p className="state error" role="alert">
                Não foi possível apagar a meta. Tente de novo.
              </p>
            )}
            {data.goals.length === 0 && !adding && <p className="state">Nenhuma meta ainda.</p>}
            <div className="bill-grid section-gap">
              {data.goals.map((g) => (
                <GoalCard
                  key={g.id}
                  g={g}
                  busy={remove.isPending}
                  onDelete={() => window.confirm(`Apagar a meta "${g.name}"?`) && remove.mutate(g.id)}
                />
              ))}
            </div>

            {adding ? (
              <div className="section-gap">
                <GoalForm
                  busy={create.isPending}
                  serverError={
                    create.error instanceof ApiError ? create.error.message : create.isError ? 'Não foi possível criar a meta.' : null
                  }
                  onSubmit={(input) => create.mutate(input)}
                  onCancel={() => {
                    setAdding(false)
                    create.reset()
                  }}
                />
              </div>
            ) : (
              <button type="button" className="btn btn-primary section-gap" onClick={() => setAdding(true)}>
                <Plus size={14} aria-hidden="true" /> Nova meta
              </button>
            )}

            <p className="tile-note section-gap">
              O andamento é calculado na hora, nada é gravado. &quot;Cabe no orçamento&quot; compara o que falta guardar por mês com a sobra
              média da projeção (renda − fixas − variáveis − parcelas), que é uma estimativa. A meta de reduzir usa a média dos meses
              anteriores à criação como base fixa.
            </p>
          </>
        )}
      </QueryState>
    </>
  )
}

function GoalCard({ g, busy, onDelete }: { g: Goal; busy: boolean; onDelete: () => void }) {
  const { label, Icon } = GOAL_KINDS[g.kind]
  const status = goalStatus(g)
  const ToneIcon = TONE_ICON[status.tone]
  // Na redução o medidor mostra o gasto do mês contra o teto (cheio = estourou); nas demais, o quanto já foi juntado.
  const ratio = progressRatio(g.current, g.target)
  const level: Tone = g.kind === 'CUT' ? (ratio >= 1 ? 'critical' : ratio >= 0.85 ? 'warning' : 'ok') : 'ok'
  const caption =
    g.kind === 'CUT'
      ? `${formatBRL(g.current)} de ${formatBRL(g.target)} no mês (${g.categoryLabel}, ${g.cutPercent}% abaixo de ${formatBRL(g.baseline)})`
      : `${formatBRL(g.current)} de ${formatBRL(g.target)} (${formatPercent(ratio)})`

  return (
    <article className="bill" style={{ '--c': 'var(--accent)' } as CSSProperties}>
      <div className="bill-top">
        <Icon size={20} aria-hidden="true" />
        <span className="bill-name">
          <strong>{g.name}</strong>
          <span className="bill-meta">{label}</span>
        </span>
      </div>
      <div
        className="meter"
        role="meter"
        aria-label={g.kind === 'CUT' ? 'Gasto do mês contra o teto' : 'Progresso da meta'}
        aria-valuemin={0}
        aria-valuemax={g.target}
        aria-valuenow={Math.min(g.current, g.target)}
        aria-valuetext={caption}
      >
        <div className={`meter-fill meter-${level}`} style={{ width: `${ratio * 100}%` }} />
      </div>
      <p className="tile-note">{caption}</p>
      <p className={`goal-status goal-${status.tone}`}>
        <ToneIcon size={14} aria-hidden="true" /> {status.text}
      </p>
      {g.kind === 'CUT' && g.history.length > 0 && (
        <ul className="goal-history" aria-label="Gasto por mês desde a criação da meta">
          {g.history.map((h, i) => (
            <li key={h.month}>
              {formatMonthShort(h.month)}: {formatBRL(h.total)}{' '}
              {i === g.history.length - 1 ? (
                '(em andamento)'
              ) : h.hit ? (
                <Check size={12} aria-label="dentro do teto" />
              ) : (
                <CircleAlert size={12} aria-label="acima do teto" />
              )}
            </li>
          ))}
        </ul>
      )}
      <div className="bill-foot">
        <button type="button" className="btn btn-small" disabled={busy} onClick={onDelete}>
          <Trash2 size={13} aria-hidden="true" /> Apagar
        </button>
      </div>
    </article>
  )
}
