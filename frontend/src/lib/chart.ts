import type { InvestmentMonth, PortfolioMonth, Totals } from '../api/types'
import { formatMonthShort } from './format'

export interface ChartRow {
  month: string
  label: string
  [series: string]: string | number | null
}

export interface Series {
  key: string
  name: string
  color: string
}

export const INCOME_EXPENSE: Series[] = [
  { key: 'income', name: 'Receitas', color: 'var(--series-1)' },
  { key: 'expense', name: 'Despesas', color: 'var(--series-2)' },
]

export const APPLIED_REDEEMED: Series[] = [
  { key: 'applied', name: 'Aplicado', color: 'var(--series-1)' },
  { key: 'redeemed', name: 'Resgatado', color: 'var(--series-2)' },
]

export const CUMULATIVE: Series[] = [{ key: 'cumulative', name: 'Líquido acumulado', color: 'var(--series-1)' }]

export const toMonthRows = (totals: Totals[]): ChartRow[] =>
  totals.map((t) => ({ month: t.month, label: formatMonthShort(t.month), income: t.income, expense: t.expense }))

export const toInvestmentRows = (months: InvestmentMonth[]): ChartRow[] =>
  months.map((m) => ({
    month: m.month,
    label: formatMonthShort(m.month),
    applied: m.applied,
    redeemed: m.redeemed,
    cumulative: m.cumulative,
  }))

export const BALANCE: Series[] = [{ key: 'balance', name: 'Saldo real', color: 'var(--series-1)' }]

export const toPortfolioRows = (months: PortfolioMonth[]): ChartRow[] =>
  months.map((m) => ({ month: m.month, label: formatMonthShort(m.month), balance: m.balance }))

/** Quantos meses têm saldo registrado (o histórico começa na primeira sincronização). */
export const knownMonths = (rows: ChartRow[], key: string) => rows.filter((r) => r[key] !== null).length
