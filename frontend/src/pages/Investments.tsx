import { useSearchParams } from 'react-router'
import { useInvestments } from '../api/client'
import { MonthFilter } from '../components/MonthFilter'
import { QueryState } from '../components/QueryState'
import { StatTile } from '../components/StatTile'
import { TrendChart } from '../components/TrendChart'
import { APPLIED_REDEEMED, CUMULATIVE, toInvestmentRows } from '../lib/chart'
import { formatBRL, formatMonthTitle } from '../lib/format'
import { summarizeInvestments } from '../lib/investments'
import { shiftMonth } from '../lib/months'
import { useMonth } from '../lib/useMonth'

const RANGES = [6, 12, 24]

export default function Investments() {
  const { month, setMonth, now } = useMonth()
  const [params, setParams] = useSearchParams()
  const requested = Number(params.get('meses'))
  const months = RANGES.includes(requested) ? requested : 12
  const query = useInvestments(shiftMonth(month, -(months - 1)), month)

  return (
    <>
      <h1 className="page-title">Investimentos até {formatMonthTitle(month)}</h1>
      <MonthFilter month={month} now={now} onChange={setMonth} />
      <div className="filters" role="group" aria-label="Período">
        {RANGES.map((n) => (
          <button
            key={n}
            type="button"
            className="btn"
            aria-pressed={months === n}
            onClick={() =>
              setParams((prev) => {
                const next = new URLSearchParams(prev)
                next.set('meses', String(n))
                return next
              })
            }
          >
            {n} meses
          </button>
        ))}
      </div>

      <QueryState query={query}>
        {(data) => {
          const stale = query.isPlaceholderData
          // A API só devolve meses a partir do primeiro lançamento de investimento.
          if (data.length === 0) {
            return (
              <p className="state">
                Nenhuma aplicação ou resgate registrado até este mês. Aplicações e resgates do Open Finance (ou registrados no chat)
                aparecem aqui.
              </p>
            )
          }
          const s = summarizeInvestments(data)
          const rows = toInvestmentRows(data)
          return (
            <>
              <div className={`tiles chart-body ${stale ? 'is-stale' : ''}`}>
                <StatTile label="Aplicado no período" value={formatBRL(s.applied)} />
                <StatTile label="Resgatado no período" value={formatBRL(s.redeemed)} />
                <StatTile label="Líquido no período" value={formatBRL(s.net)} note="Aplicado − resgatado" />
                <StatTile label="Líquido acumulado" value={formatBRL(s.cumulative)} note="Desde o primeiro lançamento" />
              </div>
              <TrendChart title="Aplicado e resgatado por mês" series={APPLIED_REDEEMED} rows={rows} stale={stale} />
              <TrendChart title="Líquido acumulado" series={CUMULATIVE} variant="area" rows={rows} stale={stale} />
            </>
          )
        }}
      </QueryState>
    </>
  )
}
