import type { InvestmentMonth, PortfolioMonth } from '../api'
import { formatMonthShort } from '../../../shared/lib/format'
import type { ChartRow, Series } from '../../../shared/lib/chart'

export const APPLIED_REDEEMED: Series[] = [
  { key: 'applied', name: 'Aplicado', color: 'var(--series-1)' },
  { key: 'redeemed', name: 'Resgatado', color: 'var(--series-2)' },
]

export const CUMULATIVE: Series[] = [{ key: 'cumulative', name: 'Líquido acumulado', color: 'var(--series-1)' }]

export const toInvestmentRows = (months: InvestmentMonth[]): ChartRow[] =>
  months.map((m) => ({
    month: m.month,
    label: formatMonthShort(m.month),
    applied: m.applied,
    redeemed: m.redeemed,
    cumulative: m.cumulative,
  }))

export const PORTFOLIO: Series[] = [
  { key: 'balance', name: 'Saldo real', color: 'var(--series-1)' },
  { key: 'estimated', name: 'Estimado', color: 'var(--series-prev)', secondary: true },
]

/**
 * Separa o saldo exato do estimado em duas séries. O primeiro mês exato também entra na série
 * estimada, para a linha cinza encontrar a azul em vez de terminar um mês antes.
 */
export function toPortfolioRows(months: PortfolioMonth[]): ChartRow[] {
  return months.map((m, i) => {
    const row: ChartRow = { month: m.month, label: formatMonthShort(m.month), balance: null, estimated: null }
    if (m.balance === null) return row
    if (m.estimated) {
      row.estimated = m.balance
    } else {
      row.balance = m.balance
      if (months[i - 1]?.estimated && months[i - 1].balance !== null) row.estimated = m.balance
    }
    return row
  })
}
