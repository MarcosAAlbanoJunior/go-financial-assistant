import type { CompareRow } from '../lib/compare'
import { deltaPercent, formatBRL, formatMonthShort, formatPercent } from '../lib/format'

interface Props {
  rows: CompareRow[]
  month: string
  previousMonth: string
  stale: boolean
}

/** Duas barras por grupo: o mês atual na cor de destaque e o anterior em cinza (ênfase, não duas categorias). */
export function CompareBars({ rows, month, previousMonth, stale }: Props) {
  if (rows.length === 0) return <p className="state">Sem despesas nos dois meses.</p>
  const max = Math.max(...rows.flatMap((r) => [r.current, r.previous]))

  return (
    <>
      <div className="legend">
        <span>
          <i style={{ background: 'var(--series-1)' }} aria-hidden="true" />
          {formatMonthShort(month)}
        </span>
        <span>
          <i style={{ background: 'var(--series-prev)' }} aria-hidden="true" />
          {formatMonthShort(previousMonth)}
        </span>
      </div>
      <ul className={`bars chart-body ${stale ? 'is-stale' : ''}`}>
        {rows.map((r) => (
          <li key={r.key}>
            <div className="bar-head">
              <span className="bar-label">{r.label}</span>
              <Change current={r.current} previous={r.previous} />
            </div>
            <Bar value={r.current} max={max} color="var(--series-1)" />
            <Bar value={r.previous} max={max} color="var(--series-prev)" />
          </li>
        ))}
      </ul>
    </>
  )
}

function Bar({ value, max, color }: { value: number; max: number; color: string }) {
  return (
    <div className="bar-row">
      <div className="bar-track thin" aria-hidden="true">
        <div className="bar-fill" style={{ width: `${(value / max) * 100}%`, background: color }} />
      </div>
      <span className="bar-amount">{formatBRL(value)}</span>
    </div>
  )
}

// Para despesa, subir é ruim. A seta e o texto dizem o mesmo que a cor.
function Change({ current, previous }: { current: number; previous: number }) {
  const pct = deltaPercent(current, previous)
  if (pct === null) return <span className="delta delta-bad">novo neste mês</span>
  if (pct === 0) return <span className="delta">sem variação</span>
  const up = pct > 0
  return (
    <span className={`delta ${up ? 'delta-bad' : 'delta-good'}`}>
      <span aria-hidden="true">{up ? '▲' : '▼'}</span> {formatPercent(Math.abs(pct))} {up ? 'a mais' : 'a menos'}
    </span>
  )
}
