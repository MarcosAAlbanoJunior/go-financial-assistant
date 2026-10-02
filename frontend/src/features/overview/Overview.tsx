import { QueryState } from '../../shared/components/QueryState'
import { MonthFilter } from '../../shared/components/MonthFilter'
import { TrendChart } from '../../shared/components/TrendChart'
import { StatTile } from '../../shared/components/StatTile'
import { INCOME_EXPENSE, toMonthRows } from '../../shared/lib/chart'
import { formatBRL, formatMonthTitle } from '../../shared/lib/format'
import { shiftMonth } from '../../shared/lib/months'
import { useMonth } from '../../shared/lib/useMonth'
import { useSummary, useTimeseries } from '../../shared/api/totals'

export default function Overview() {
  const { month, setMonth, now } = useMonth()
  const summary = useSummary(month)
  const series = useTimeseries(shiftMonth(month, -11), month)

  return (
    <>
      <h1 className="page-title">{formatMonthTitle(month)}</h1>
      <MonthFilter month={month} now={now} onChange={setMonth} warnPartial />

      <QueryState query={summary}>{(data) => <Tiles data={data} stale={summary.isPlaceholderData} partial={month === now} />}</QueryState>

      <QueryState query={series}>
        {(data) => <TrendChart series={INCOME_EXPENSE} title="Receitas e despesas por mês" rows={toMonthRows(data)} stale={series.isPlaceholderData} />}
      </QueryState>
    </>
  )
}

function Tiles({ data, stale, partial }: { data: NonNullable<ReturnType<typeof useSummary>['data']>; stale: boolean; partial: boolean }) {
  const { current: c, previous: p } = data
  const balance = c.income - c.expense
  const invested = c.applied - c.redeemed

  return (
    <div className={`tiles chart-body ${stale ? 'is-stale' : ''}`}>
      <StatTile
        label="Receitas"
        value={formatBRL(c.income)}
        compare={partial ? undefined : { current: c.income, previous: p.income, previousMonth: p.month, upIsGood: true }}
      />
      <StatTile
        label="Despesas"
        value={formatBRL(c.expense)}
        compare={partial ? undefined : { current: c.expense, previous: p.expense, previousMonth: p.month, upIsGood: false }}
      />
      <StatTile
        label="Saldo do mês"
        value={formatBRL(balance)}
        compare={partial ? undefined : { current: balance, previous: p.income - p.expense, previousMonth: p.month, upIsGood: true }}
      />
      <StatTile
        label="Em conta"
        to="/"
        value={data.bankBalance === null ? '—' : formatBRL(data.bankBalance)}
        note={data.bankBalance === null ? 'Sem contas do Open Finance' : 'Contas correntes, hoje'}
      />
      <StatTile
        label="Investido no mês"
        value={formatBRL(invested)}
        note={`Aplicado ${formatBRL(c.applied)} · Resgatado ${formatBRL(c.redeemed)}`}
      />
    </div>
  )
}
