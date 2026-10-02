import { useSummary, useTimeseries } from '../api/client'
import { MonthFilter } from '../components/MonthFilter'
import { TrendChart } from '../components/TrendChart'
import { StatTile } from '../components/StatTile'
import { INCOME_EXPENSE, toMonthRows } from '../lib/chart'
import { formatBRL, formatMonthTitle } from '../lib/format'
import { shiftMonth } from '../lib/months'
import { useMonth } from '../lib/useMonth'

export default function Overview() {
  const { month, setMonth, now } = useMonth()
  const summary = useSummary(month)
  const series = useTimeseries(shiftMonth(month, -11), month)

  return (
    <>
      <h1 className="page-title">{formatMonthTitle(month)}</h1>
      <MonthFilter month={month} now={now} onChange={setMonth} />

      {summary.isError && !summary.data ? (
        <div className="state error" role="alert">
          Não foi possível carregar o resumo.{' '}
          <button type="button" className="btn" onClick={() => summary.refetch()}>
            Tentar de novo
          </button>
        </div>
      ) : summary.data ? (
        <Tiles data={summary.data} stale={summary.isPlaceholderData} />
      ) : (
        <p className="state">Carregando…</p>
      )}

      {series.isError && !series.data ? (
        <div className="state error" role="alert">
          Não foi possível carregar o histórico.{' '}
          <button type="button" className="btn" onClick={() => series.refetch()}>
            Tentar de novo
          </button>
        </div>
      ) : (
        series.data && <TrendChart series={INCOME_EXPENSE} title="Receitas e despesas por mês" rows={toMonthRows(series.data)} stale={series.isPlaceholderData} />
      )}
    </>
  )
}

function Tiles({ data, stale }: { data: NonNullable<ReturnType<typeof useSummary>['data']>; stale: boolean }) {
  const { current: c, previous: p } = data
  const balance = c.income - c.expense
  const invested = c.applied - c.redeemed

  return (
    <div className={`tiles chart-body ${stale ? 'is-stale' : ''}`}>
      <StatTile
        label="Receitas"
        value={formatBRL(c.income)}
        compare={{ current: c.income, previous: p.income, previousMonth: p.month, upIsGood: true }}
      />
      <StatTile
        label="Despesas"
        value={formatBRL(c.expense)}
        compare={{ current: c.expense, previous: p.expense, previousMonth: p.month, upIsGood: false }}
      />
      <StatTile
        label="Saldo do mês"
        value={formatBRL(balance)}
        compare={{ current: balance, previous: p.income - p.expense, previousMonth: p.month, upIsGood: true }}
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
