import { useState } from 'react'
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import type { TooltipContentProps } from 'recharts'
import type { MonthRow } from '../lib/chart'
import { formatBRL, formatBRLCompact, formatMonthLong } from '../lib/format'

const SERIES = [
  { key: 'income', name: 'Receitas', color: 'var(--series-1)' },
  { key: 'expense', name: 'Despesas', color: 'var(--series-2)' },
] as const

interface Props {
  rows: MonthRow[]
  /** Dados antigos mantidos na tela enquanto os novos carregam. */
  stale: boolean
}

export function MonthlyChart({ rows, stale }: Props) {
  const [asTable, setAsTable] = useState(false)

  return (
    <section className="card chart-card" aria-labelledby="monthly-title">
      <div className="chart-head">
        <h2 className="chart-title" id="monthly-title">
          Receitas e despesas por mês
        </h2>
        <button type="button" className="btn" aria-pressed={asTable} onClick={() => setAsTable(!asTable)}>
          {asTable ? 'Ver gráfico' : 'Ver como tabela'}
        </button>
      </div>
      <div className="legend">
        {SERIES.map((s) => (
          <span key={s.key}>
            <i style={{ background: s.color }} aria-hidden="true" />
            {s.name}
          </span>
        ))}
      </div>

      <div className={`chart-body ${stale ? 'is-stale' : ''}`}>
        {asTable ? (
          <MonthlyTable rows={rows} />
        ) : (
          <div role="img" aria-label="Gráfico de colunas com receitas e despesas dos últimos 12 meses; use 'Ver como tabela' para os valores">
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={rows} margin={{ top: 8, right: 8, bottom: 0, left: 0 }} barGap={2} barCategoryGap="20%">
                <CartesianGrid vertical={false} stroke="var(--grid)" />
                <XAxis dataKey="label" tickLine={false} axisLine={{ stroke: 'var(--axis)' }} tick={{ fill: 'var(--text-muted)', fontSize: 12 }} />
                <YAxis
                  tickFormatter={formatBRLCompact}
                  tickLine={false}
                  axisLine={false}
                  width={72}
                  tick={{ fill: 'var(--text-muted)', fontSize: 12 }}
                />
                <Tooltip content={ChartTooltip} cursor={{ fill: 'var(--grid)', opacity: 0.5 }} />
                {SERIES.map((s) => (
                  <Bar key={s.key} dataKey={s.key} name={s.name} fill={s.color} maxBarSize={16} radius={[4, 4, 0, 0]} />
                ))}
              </BarChart>
            </ResponsiveContainer>
          </div>
        )}
      </div>
    </section>
  )
}

// React escapa o texto: nomes e valores vindos da API nunca entram como HTML.
function ChartTooltip({ active, payload }: TooltipContentProps) {
  if (!active || !payload?.length) return null
  const row = payload[0].payload as MonthRow
  return (
    <div className="tooltip">
      <p className="tooltip-title">{formatMonthLong(row.month)}</p>
      {payload.map((p) => (
        <div className="tooltip-row" key={String(p.dataKey)}>
          <i style={{ borderColor: p.color }} aria-hidden="true" />
          <strong>{formatBRL(Number(p.value))}</strong>
          <span>{p.name}</span>
        </div>
      ))}
    </div>
  )
}

function MonthlyTable({ rows }: { rows: MonthRow[] }) {
  return (
    <table className="data">
      <thead>
        <tr>
          <th scope="col">Mês</th>
          <th scope="col">Receitas</th>
          <th scope="col">Despesas</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.month}>
            <th scope="row">{r.label}</th>
            <td>{formatBRL(r.income)}</td>
            <td>{formatBRL(r.expense)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
