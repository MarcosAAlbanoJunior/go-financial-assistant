import { useSearchParams } from 'react-router'
import { useSummary, useTimeseries } from '../api/client'
import { MonthlyChart } from '../components/MonthlyChart'
import { StatTile } from '../components/StatTile'
import { toMonthRows } from '../lib/chart'
import { formatBRL, formatMonthTitle } from '../lib/format'
import { currentMonth, isMonth, shiftMonth } from '../lib/months'

export default function Overview() {
  const [params, setParams] = useSearchParams()
  const now = currentMonth()
  const requested = params.get('mes')
  const month = isMonth(requested) ? requested : now

  const summary = useSummary(month)
  const series = useTimeseries(shiftMonth(month, -11), month)

  const setMonth = (m: string) => {
    if (isMonth(m)) setParams({ mes: m })
  }

  return (
    <>
      <h1 className="page-title">{formatMonthTitle(month)}</h1>

      <div className="filters">
        <button type="button" className="btn" aria-label="Mês anterior" onClick={() => setMonth(shiftMonth(month, -1))}>
          ←
        </button>
        <input
          type="month"
          aria-label="Mês"
          value={month}
          max={now}
          onChange={(e) => setMonth(e.target.value)}
        />
        <button
          type="button"
          className="btn"
          aria-label="Próximo mês"
          disabled={month >= now}
          onClick={() => setMonth(shiftMonth(month, 1))}
        >
          →
        </button>
      </div>

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
        series.data && <MonthlyChart rows={toMonthRows(series.data)} stale={series.isPlaceholderData} />
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
