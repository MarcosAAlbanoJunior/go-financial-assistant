import type { CSSProperties } from 'react'
import { CategoryChip } from './CategoryChip'
import { categoryVisual } from '../lib/categoryVisual'
import { formatBRL, formatPercent } from '../lib/format'

interface Item {
  key: string
  label: string
  total: number
}

/**
 * Ranking de gastos em barras horizontais. Cada valor está escrito ao lado do rótulo, então nada
 * depende de hover. Com categories, cada barra usa a cor e o ícone da categoria (a mesma em todas
 * as telas); sem, os grupos não têm ordem natural e valem uma cor só.
 */
export function RankedBars({
  items,
  stale,
  empty = 'Sem despesas neste mês.',
  categories = false,
}: {
  items: Item[]
  stale: boolean
  empty?: string
  categories?: boolean
}) {
  if (items.length === 0) return <p className="state">{empty}</p>

  const sum = items.reduce((acc, i) => acc + i.total, 0)
  const max = Math.max(...items.map((i) => i.total))

  return (
    <ul className={`bars chart-body ${stale ? 'is-stale' : ''}`}>
      {items.map((i) => (
        <li key={i.key} style={categories ? ({ '--cat': categoryVisual(i.key).color } as CSSProperties) : undefined}>
          <div className="bar-head">
            <span className="bar-label">
              {categories && <CategoryChip category={i.key} size={14} />}
              {i.label}
            </span>
            <span className="bar-value">
              <strong>{formatBRL(i.total)}</strong> <span className="bar-share">{formatPercent(i.total / sum)}</span>
            </span>
          </div>
          <div className="bar-track" aria-hidden="true">
            <div className="bar-fill" style={{ width: `${(i.total / max) * 100}%`, background: categories ? 'var(--cat)' : undefined }} />
          </div>
        </li>
      ))}
    </ul>
  )
}
