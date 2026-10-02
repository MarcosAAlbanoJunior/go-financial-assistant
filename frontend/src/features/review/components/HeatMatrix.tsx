import type { CSSProperties } from 'react'
import { Link } from 'react-router'
import { categoryVisual } from '../../../shared/lib/categoryVisual'
import { formatBRL, formatBRLWhole, formatMonthLong, formatMonthShort } from '../../../shared/lib/format'
import { heatLevels, transactionsLink } from '../lib/review'
import { CategoryChip } from '../../../shared/components/CategoryChip'
import type { ReviewRow } from '../api'

/**
 * Mapa de calor categoria x mês. Um matiz só: quanto mais escura a célula, mais a categoria gastou
 * naquele mês em relação aos outros meses dela. O valor sempre está escrito, e cada célula leva às transações.
 */
export function HeatMatrix({ months, rows, stale }: { months: string[]; rows: ReviewRow[]; stale?: boolean }) {
  if (rows.length === 0) return <p className="state">Sem despesas nesses meses.</p>
  return (
    <div className={`heat-scroll${stale ? ' stale' : ''}`}>
      <table className="heat">
        <caption className="sr-only">Despesas por categoria e mês. Cada valor leva às transações daquele mês e categoria.</caption>
        <thead>
          <tr>
            <th scope="col">Categoria</th>
            {months.map((m) => (
              <th scope="col" key={m}>
                {formatMonthShort(m)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => {
            const levels = heatLevels(row.values)
            return (
              <tr key={row.category}>
                <th scope="row" className="heat-cat" style={{ '--cat': categoryVisual(row.category).color } as CSSProperties}>
                  <span className="heat-cat-in">
                    <CategoryChip category={row.category} size={14} />
                    {row.categoryLabel}
                  </span>
                </th>
                {row.values.map((value, i) => {
                  const level = levels[i]
                  if (level === null) return <td key={months[i]} className="heat-empty" aria-label="sem gastos">–</td>
                  return (
                    <td key={months[i]} style={{ '--h': level } as CSSProperties}>
                      <Link
                        to={transactionsLink(months[i], row.category)}
                        title={`${row.categoryLabel} em ${formatMonthLong(months[i])}: ${formatBRL(value)}`}
                        aria-label={`${row.categoryLabel}, ${formatMonthLong(months[i])}: ${formatBRL(value)}. Ver transações`}
                      >
                        {formatBRLWhole(value)}
                      </Link>
                    </td>
                  )
                })}
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
