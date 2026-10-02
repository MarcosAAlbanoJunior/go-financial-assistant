import { useState, type FormEvent } from 'react'
import { CUT_CATEGORIES, GOAL_KINDS, GOAL_ORDER, emptyGoalForm, toGoalInput } from '../lib/goals'
import type { GoalInput } from '../api/types'
import { currentMonth, shiftMonth } from '../lib/months'

interface Props {
  busy: boolean
  /** Erro devolvido pela API (ex.: categoria sem histórico). */
  serverError: string | null
  onSubmit: (input: GoalInput) => void
  onCancel: () => void
}

/** Formulário de uma nova meta; a validação daqui espelha a da API. */
export function GoalForm({ busy, serverError, onSubmit, onCancel }: Props) {
  const thisMonth = currentMonth()
  const [f, setF] = useState(() => emptyGoalForm(shiftMonth(thisMonth, 1)))
  const [error, setError] = useState<string | null>(null)
  const set = (changes: Partial<typeof f>) => setF((prev) => ({ ...prev, ...changes }))

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const result = toGoalInput(f, thisMonth)
    if ('error' in result) setError(result.error)
    else {
      setError(null)
      onSubmit(result.input)
    }
  }

  return (
    <form className="card scenario-form" onSubmit={handleSubmit} aria-label="Nova meta">
      <h2 className="chart-title">Nova meta</h2>
      <div className="view-toggle" role="group" aria-label="Tipo de meta">
        {GOAL_ORDER.map((k) => (
          <button key={k} type="button" className="btn" aria-pressed={f.kind === k} onClick={() => set({ kind: k })}>
            {GOAL_KINDS[k].label}
          </button>
        ))}
      </div>
      <p className="tile-note">{GOAL_KINDS[f.kind].hint}.</p>

      <div className="form-grid">
        <label>
          Nome
          <input type="text" maxLength={60} required placeholder="Ex.: Viagem" value={f.name} onChange={(e) => set({ name: e.target.value })} />
        </label>
        {f.kind === 'SAVE' && (
          <>
            <label>
              Valor a juntar (R$)
              <input type="number" min={0} step="0.01" required value={f.amount} onChange={(e) => set({ amount: e.target.value })} />
            </label>
            <label>
              Até
              <input type="month" required min={shiftMonth(thisMonth, 1)} value={f.date} onChange={(e) => set({ date: e.target.value })} />
            </label>
          </>
        )}
        {f.kind === 'CUT' && (
          <>
            <label>
              Categoria
              <select value={f.category} onChange={(e) => set({ category: e.target.value })}>
                {CUT_CATEGORIES.map((c) => (
                  <option key={c.value} value={c.value}>
                    {c.label}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Reduzir em (%)
              <input type="number" min={1} max={90} step={1} required value={f.percent} onChange={(e) => set({ percent: e.target.value })} />
            </label>
          </>
        )}
        {f.kind === 'RESERVE' && (
          <label>
            Meses de despesas fixas
            <input type="number" min={1} max={36} step={1} required value={f.months} onChange={(e) => set({ months: e.target.value })} />
          </label>
        )}
      </div>

      {(error ?? serverError) && (
        <p className="state error" role="alert">
          {error ?? serverError}
        </p>
      )}
      <div className="form-actions">
        <button type="submit" className="btn btn-primary" disabled={busy}>
          Criar meta
        </button>
        <button type="button" className="btn" onClick={onCancel}>
          Cancelar
        </button>
      </div>
    </form>
  )
}
