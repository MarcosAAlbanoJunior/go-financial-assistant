import { shiftMonth } from '../lib/months'

interface Props {
  month: string
  now: string
  onChange: (month: string) => void
}

/** Filtro de período: uma linha acima de tudo o que ele controla. */
export function MonthFilter({ month, now, onChange }: Props) {
  return (
    <div className="filters">
      <button type="button" className="btn" aria-label="Mês anterior" onClick={() => onChange(shiftMonth(month, -1))}>
        ←
      </button>
      <input type="month" aria-label="Mês" value={month} max={now} onChange={(e) => onChange(e.target.value)} />
      <button
        type="button"
        className="btn"
        aria-label="Próximo mês"
        disabled={month >= now}
        onClick={() => onChange(shiftMonth(month, 1))}
      >
        →
      </button>
    </div>
  )
}
