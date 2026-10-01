import { useSearchParams } from 'react-router'
import { useInvestments, usePortfolio, usePortfolioHistory } from '../api/client'
import { MonthFilter } from '../components/MonthFilter'
import { QueryState } from '../components/QueryState'
import { RankedBars } from '../components/RankedBars'
import { StatTile } from '../components/StatTile'
import { TrendChart } from '../components/TrendChart'
import { APPLIED_REDEEMED, CUMULATIVE, knownMonths, PORTFOLIO, toInvestmentRows, toPortfolioRows } from '../lib/chart'
import { formatBRL, formatMonthLong, formatMonthTitle } from '../lib/format'
import { summarizeInvestments } from '../lib/investments'
import { groupPositions } from '../lib/portfolio'
import { shiftMonth } from '../lib/months'
import { useMonth } from '../lib/useMonth'

const RANGES = [6, 12, 24]

export default function Investments() {
  const { month, setMonth, now } = useMonth()
  const [params, setParams] = useSearchParams()
  const requested = Number(params.get('meses'))
  const months = RANGES.includes(requested) ? requested : 12
  const from = shiftMonth(month, -(months - 1))
  const query = useInvestments(from, month)
  const portfolio = usePortfolio()
  const history = usePortfolioHistory(from, month)

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

      <h2 className="section-title">Patrimônio investido</h2>
      <QueryState query={portfolio}>
        {(p) => {
          const groups = groupPositions(p.positions)
          return groups.length === 0 ? (
            <p className="state">
              Nenhuma posição de investimento sincronizada. Conecte uma instituição com investimentos no Open Finance (veja o
              README).
            </p>
          ) : (
            <>
              <div className="tiles">
                <StatTile label="Saldo atual" value={formatBRL(p.total)} note={`${groups.length} produto(s), líquido de impostos`} />
              </div>
              <div className="grid-2 section-gap">
                <section className="card" aria-labelledby="h-type">
                  <h2 className="chart-title" id="h-type">
                    Por tipo
                  </h2>
                  <RankedBars items={p.byType} stale={false} empty="Sem posições." />
                </section>
                <section className="card" aria-labelledby="h-pos">
                  <h2 className="chart-title" id="h-pos">
                    Posições
                  </h2>
                  <div className="table-scroll">
                    <table className="data tx">
                      <thead>
                        <tr>
                          <th scope="col">Produto</th>
                          <th scope="col">Saldo</th>
                        </tr>
                      </thead>
                      <tbody>
                        {groups.map((g) => (
                          <tr key={g.key}>
                            <td className="left">
                              {g.name}
                              <span className="tx-meta always">
                                {[g.typeLabel, g.subtype, g.count > 1 ? `${g.count} aplicações` : ''].filter(Boolean).join(' · ')}
                              </span>
                            </td>
                            <td className="amount">{formatBRL(g.balance)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </section>
              </div>
              <QueryState query={history}>
                {(h) => {
                  const rows = toPortfolioRows(h)
                  if (knownMonths(rows, 'balance') + knownMonths(rows, 'estimated') === 0) return null
                  const firstExact = rows.find((r) => r.balance !== null)
                  const hasEstimate = knownMonths(rows, 'estimated') > 0
                  return (
                    <>
                      <TrendChart title="Saldo ao longo do tempo" series={hasEstimate ? PORTFOLIO : PORTFOLIO.slice(0, 1)} variant="area" rows={rows} stale={history.isPlaceholderData} />
                      <p className="notice" role="note">
                        <span aria-hidden="true">ⓘ</span>{' '}
                        {hasEstimate && firstExact ? (
                          <>
                            Os meses anteriores a {formatMonthLong(firstExact.month)} são uma <strong>estimativa</strong>: foram reconstruídos
                            a partir das aplicações e resgates de cada produto e não descontam os rendimentos do período, então tendem a ficar
                            um pouco acima do real. De {formatMonthLong(firstExact.month)} em diante, o valor é o saldo exato informado pelo
                            banco (a partir da conexão com o Pluggy).
                          </>
                        ) : (
                          <>O saldo exato começa na primeira sincronização com o banco: o Pluggy não informa saldos passados.</>
                        )}
                      </p>
                    </>
                  )
                }}
              </QueryState>
            </>
          )
        }}
      </QueryState>

      <h2 className="section-title">Aplicações e resgates</h2>
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
