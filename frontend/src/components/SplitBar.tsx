import type { CSSProperties } from 'react'
import { formatBRL, formatPercent } from '../lib/format'

export interface Part {
  label: string
  value: number
  color: string
}

/** Uma barra dividida em partes com legenda: os valores e percentuais ficam escritos fora dos segmentos. */
export function SplitBar({ parts, stale }: { parts: Part[]; stale: boolean }) {
  const total = parts.reduce((s, p) => s + p.value, 0)
  if (total <= 0) return <p className="state">Sem despesas neste mês.</p>

  return (
    <div className={`chart-body ${stale ? 'is-stale' : ''}`}>
      <div className="split-bar" role="img" aria-label={parts.map((p) => `${p.label} ${formatPercent(p.value / total)}`).join(', ')}>
        {parts
          .filter((p) => p.value > 0)
          .map((p) => (
            <span key={p.label} style={{ flex: p.value, '--c': p.color } as CSSProperties} />
          ))}
      </div>
      <div className="split-legend">
        {parts.map((p) => (
          <span key={p.label} style={{ '--c': p.color } as CSSProperties}>
            <i aria-hidden="true" />
            {p.label}: <strong>{formatBRL(p.value)}</strong> ({formatPercent(p.value / total)})
          </span>
        ))}
      </div>
    </div>
  )
}
