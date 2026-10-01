import { deltaPercent, formatMonthShort, formatPercent } from '../lib/format'

interface Props {
  label: string
  value: string
  note?: string
  /** Comparação com o mês anterior; omitida quando o cartão não tem série (ex.: saldo em conta). */
  compare?: { current: number; previous: number; previousMonth: string; upIsGood: boolean }
}

export function StatTile({ label, value, note, compare }: Props) {
  return (
    <div className="card">
      <p className="tile-label">{label}</p>
      <p className="tile-value">{value}</p>
      {note && <p className="tile-note">{note}</p>}
      {compare && <Delta {...compare} />}
    </div>
  )
}

// A cor reforça, mas a seta e o texto carregam o sentido: nunca só a cor.
function Delta({ current, previous, previousMonth, upIsGood }: NonNullable<Props['compare']>) {
  const pct = deltaPercent(current, previous)
  const vs = `vs. ${formatMonthShort(previousMonth)}`
  if (pct === null) return <p className="delta">Sem base em {formatMonthShort(previousMonth)}</p>
  if (pct === 0) return <p className="delta">Sem variação {vs}</p>

  const up = pct > 0
  const good = up === upIsGood
  return (
    <p className={`delta ${good ? 'delta-good' : 'delta-bad'}`}>
      <span aria-hidden="true">{up ? '▲' : '▼'}</span> {formatPercent(Math.abs(pct))} {up ? 'a mais' : 'a menos'} {vs}
    </p>
  )
}
