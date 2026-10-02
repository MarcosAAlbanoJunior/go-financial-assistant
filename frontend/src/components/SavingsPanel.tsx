import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Check, CircleAlert, Clock, Undo2 } from 'lucide-react'
import { setDecision, useSavings } from '../api/client'
import { cleanLabel } from '../lib/budget'
import { formatBRL } from '../lib/format'
import { savingStatus } from '../lib/savings'
import { CategoryChip } from './CategoryChip'
import { StatTile } from './StatTile'

const TONE_ICON = { ok: Check, warning: Clock, critical: CircleAlert }

/** O que você já deixou de gastar com as contas que marcou como canceladas, conferido mês a mês. */
export function SavingsPanel() {
  const savings = useSavings()
  const queryClient = useQueryClient()
  const undo = useMutation({
    mutationFn: ({ kind, key }: { kind: Parameters<typeof setDecision>[0]; key: string }) => setDecision(kind, key, '', false),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['savings'] })
      queryClient.invalidateQueries({ queryKey: ['review'] })
      queryClient.invalidateQueries({ queryKey: ['coach-preview'] })
    },
  })
  const data = savings.data
  if (!data || data.decisions.length === 0) return null

  return (
    <section className="card section-gap" aria-labelledby="savings-title">
      <h2 className="chart-title" id="savings-title">
        Economia realizada
      </h2>
      <p className="chart-sub">
        O app confere, mês a mês, se as contas que você marcou como canceladas realmente deixaram de ser cobradas.
      </p>
      <div className="tiles">
        <StatTile label="Já deixou de gastar" value={formatBRL(data.realized)} />
        <StatTile label="Economia por mês" value={formatBRL(data.perMonth)} note={`${formatBRL(data.perYear)} por ano, nas confirmadas`} />
      </div>
      <ul className="saving-list">
        {data.decisions.map((d) => {
          const status = savingStatus(d)
          const Icon = TONE_ICON[status.tone]
          return (
            <li key={d.kind + d.key}>
              <CategoryChip category={d.category} size={16} />
              <span className="saving-main">
                <strong>{cleanLabel(d.label)}</strong> <span className="bill-meta">({formatBRL(d.monthly)}/mês)</span>
                <span className={`goal-status goal-${status.tone}`}>
                  <Icon size={14} aria-hidden="true" /> {status.text}
                </span>
              </span>
              <button
                type="button"
                className="btn btn-small"
                disabled={undo.isPending}
                onClick={() => undo.mutate({ kind: d.kind, key: d.key })}
              >
                <Undo2 size={13} aria-hidden="true" /> Desfazer
              </button>
            </li>
          )
        })}
      </ul>
      {undo.isError && (
        <p className="state error" role="alert">
          Não foi possível desfazer. Tente de novo.
        </p>
      )}
    </section>
  )
}
