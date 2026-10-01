import { formatBRL, formatPercent } from '../lib/format'

interface Item {
  key: string
  label: string
  total: number
}

/**
 * Ranking de gastos em barras horizontais. Uma série só, então uma cor só (os grupos não têm
 * ordem natural). Cada valor está escrito ao lado do rótulo, então nada depende de hover.
 */
export function RankedBars({ items, stale, empty = 'Sem despesas neste mês.' }: { items: Item[]; stale: boolean; empty?: string }) {
  if (items.length === 0) return <p className="state">{empty}</p>

  const sum = items.reduce((acc, i) => acc + i.total, 0)
  const max = Math.max(...items.map((i) => i.total))

  return (
    <ul className={`bars chart-body ${stale ? 'is-stale' : ''}`}>
      {items.map((i) => (
        <li key={i.key}>
          <div className="bar-head">
            <span className="bar-label">{i.label}</span>
            <span className="bar-value">
              <strong>{formatBRL(i.total)}</strong> <span className="bar-share">{formatPercent(i.total / sum)}</span>
            </span>
          </div>
          <div className="bar-track" aria-hidden="true">
            <div className="bar-fill" style={{ width: `${(i.total / max) * 100}%` }} />
          </div>
        </li>
      ))}
    </ul>
  )
}
