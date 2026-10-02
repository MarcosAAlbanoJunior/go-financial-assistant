import { useSearchParams } from 'react-router'
import { CompareBars } from './components/CompareBars'
import { MonthFilter } from '../../shared/components/MonthFilter'
import { TrendChart } from '../../shared/components/TrendChart'
import { QueryState } from '../../shared/components/QueryState'
import { INCOME_EXPENSE, toMonthRows } from '../../shared/lib/chart'
import { mergeBreakdowns } from './lib/compare'
import { formatMonthTitle } from '../../shared/lib/format'
import { shiftMonth } from '../../shared/lib/months'
import { useMonth } from '../../shared/lib/useMonth'
import { useBreakdown, useTimeseries } from '../../shared/api/totals'

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
      <MonthFilter month={month} now={now} onChange={setMonth} warnPartial />

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
          <TrendChart series={INCOME_EXPENSE} title={`Evolução em ${months} meses`} variant="lines" rows={toMonthRows(totals)} stale={series.isPlaceholderData} />
        )}
      </QueryState>
    </>
  )
}
