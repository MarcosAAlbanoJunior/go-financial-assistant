import { useSearchParams } from 'react-router'
import { useBreakdown, useTimeseries } from '../api/client'
import { CompareBars } from '../components/CompareBars'
import { MonthFilter } from '../components/MonthFilter'
import { MonthlyChart } from '../components/MonthlyChart'
import { QueryState } from '../components/QueryState'
import { toMonthRows } from '../lib/chart'
import { mergeBreakdowns } from '../lib/compare'
import { formatMonthTitle } from '../lib/format'
import { shiftMonth } from '../lib/months'
import { useMonth } from '../lib/useMonth'

const RANGES = [6, 12, 24]

export default function Compare() {
  const { month, setMonth, now } = useMonth()
  const [params, setParams] = useSearchParams()
  const requested = Number(params.get('meses'))
  const months = RANGES.includes(requested) ? requested : 12
  const previousMonth = shiftMonth(month, -1)

  const current = useBreakdown(month, 'category')
  const previous = useBreakdown(previousMonth, 'category')
  const series = useTimeseries(shiftMonth(month, -(months - 1)), month)

  return (
    <>
      <h1 className="page-title">Comparações até {formatMonthTitle(month)}</h1>
      <MonthFilter month={month} now={now} onChange={setMonth} />

      <section className="card" aria-labelledby="cmp-title">
        <h2 className="chart-title" id="cmp-title">
          Despesas por categoria: mês escolhido e mês anterior
        </h2>
        <QueryState query={current}>
          {(cur) => (
            <QueryState query={previous}>
              {(prev) => (
                <CompareBars
                  rows={mergeBreakdowns(cur, prev)}
                  month={month}
                  previousMonth={previousMonth}
                  stale={current.isPlaceholderData || previous.isPlaceholderData}
                />
              )}
            </QueryState>
          )}
        </QueryState>
      </section>

      <div className="filters section-gap" role="group" aria-label="Período da evolução">
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
      <QueryState query={series}>
        {(totals) => (
          <MonthlyChart title={`Evolução em ${months} meses`} variant="lines" rows={toMonthRows(totals)} stale={series.isPlaceholderData} />
        )}
      </QueryState>
    </>
  )
}
