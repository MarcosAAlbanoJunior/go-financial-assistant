import { useId, useState } from 'react'
import { Area, AreaChart, Bar, BarChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import type { TooltipContentProps } from 'recharts'
import { knownMonths, type ChartRow, type Series } from '../lib/chart'
import { formatBRL, formatBRLCompact, formatMonthLong } from '../lib/format'

const MARGIN = { top: 8, right: 8, bottom: 0, left: 0 }

interface Props {
  title: string
  series: Series[]
  /** Colunas comparam meses lado a lado; linhas e área mostram a tendência de períodos longos. */
  variant?: 'bars' | 'lines' | 'area'
  rows: ChartRow[]
  /** Dados antigos mantidos na tela enquanto os novos carregam. */
  stale: boolean
}

export function TrendChart({ title, series, variant = 'bars', rows, stale }: Props) {
  const [asTable, setAsTable] = useState(false)
  const titleId = useId()

  const axes = [
    <CartesianGrid key="grid" vertical={false} stroke="var(--grid)" />,
    <XAxis key="x" dataKey="label" tickLine={false} axisLine={{ stroke: 'var(--axis)' }} tick={{ fill: 'var(--text-muted)', fontSize: 12 }} />,
    <YAxis key="y" tickFormatter={formatBRLCompact} tickLine={false} axisLine={false} width={72} tick={{ fill: 'var(--text-muted)', fontSize: 12 }} />,
    <Tooltip key="tip" content={(props) => <ChartTooltip {...props} series={series} />} cursor={variant === 'bars' ? { fill: 'var(--grid)', opacity: 0.5 } : { stroke: 'var(--axis)' }} />,
  ]
  const activeDot = { r: 4, stroke: 'var(--surface)', strokeWidth: 2 }
  // Com um único ponto não há linha para ver: mostra o ponto.
  const dotFor = (key: string) => (knownMonths(rows, key) === 1 ? { r: 4, stroke: 'var(--surface)', strokeWidth: 2 } : false)

  return (
    <section className="card chart-card" aria-labelledby={titleId}>
      <div className="chart-head">
        <h2 className="chart-title" id={titleId}>
          {title}
        </h2>
        <button type="button" className="btn" aria-pressed={asTable} onClick={() => setAsTable(!asTable)}>
          {asTable ? 'Ver gráfico' : 'Ver como tabela'}
        </button>
      </div>
      {/* Uma série só não precisa de legenda: o título já diz o que está plotado. */}
      {series.length > 1 && (
        <div className="legend">
          {series.map((s) => (
            <span key={s.key}>
              <i style={{ background: s.color }} aria-hidden="true" />
              {s.name}
            </span>
          ))}
        </div>
      )}

      <div className={`chart-body ${stale ? 'is-stale' : ''}`}>
        {asTable ? (
          <DataTable rows={rows} series={series} />
        ) : (
          <div role="img" aria-label={`${title}; use 'Ver como tabela' para os valores`}>
            <ResponsiveContainer width="100%" height={300}>
              {variant === 'bars' ? (
                <BarChart data={rows} margin={MARGIN} barGap={2} barCategoryGap="20%">
                  {axes}
                  {series.map((s) => (
                    <Bar key={s.key} dataKey={s.key} name={s.name} fill={s.color} maxBarSize={16} radius={[4, 4, 0, 0]} />
                  ))}
                </BarChart>
              ) : variant === 'lines' ? (
                <LineChart data={rows} margin={MARGIN}>
                  {axes}
                  {series.map((s) => (
                    <Line key={s.key} dataKey={s.key} name={s.name} stroke={s.color} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" dot={dotFor(s.key)} activeDot={activeDot} />
                  ))}
                </LineChart>
              ) : (
                <AreaChart data={rows} margin={MARGIN}>
                  {axes}
                  {series.map((s) => (
                    <Area key={s.key} dataKey={s.key} name={s.name} stroke={s.color} strokeWidth={2} fill={s.color} fillOpacity={0.1} dot={dotFor(s.key)} activeDot={activeDot} />
                  ))}
                </AreaChart>
              )}
            </ResponsiveContainer>
          </div>
        )}
      </div>
    </section>
  )
}

// React escapa o texto: nomes e valores vindos da API nunca entram como HTML.
function ChartTooltip({ active, payload, series }: TooltipContentProps & { series: Series[] }) {
  if (!active || !payload?.length) return null
  const row = payload[0].payload as ChartRow
  const secondary = new Set(series.filter((s) => s.secondary).map((s) => s.key))
  const hasPrimary = series.some((s) => !s.secondary && row[s.key] !== null && row[s.key] !== undefined)
  return (
    <div className="tooltip">
      <p className="tooltip-title">{formatMonthLong(row.month)}</p>
      {payload
        .filter((p) => p.value !== null && p.value !== undefined && !(hasPrimary && secondary.has(String(p.dataKey))))
        .map((p) => (
        <div className="tooltip-row" key={String(p.dataKey)}>
          <i style={{ borderColor: p.color }} aria-hidden="true" />
          <strong>{formatBRL(Number(p.value))}</strong>
          <span>{p.name}</span>
        </div>
      ))}
    </div>
  )
}

function DataTable({ rows, series }: { rows: ChartRow[]; series: Series[] }) {
  return (
    <table className="data">
      <thead>
        <tr>
          <th scope="col">Mês</th>
          {series.map((s) => (
            <th scope="col" key={s.key}>
              {s.name}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.month}>
            <th scope="row">{r.label}</th>
            {series.map((s) => (
              <td key={s.key}>{r[s.key] === null ? '—' : formatBRL(Number(r[s.key]))}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  )
}
