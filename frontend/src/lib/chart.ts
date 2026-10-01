import type { BudgetMonth, InvestmentMonth, PortfolioMonth, Totals } from '../api/types'
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
  /** Série de apoio (ex.: estimativa): no tooltip só aparece quando as demais estão vazias na linha. */
  secondary?: boolean
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

/** Quantos meses têm saldo registrado (o histórico começa na primeira sincronização). */
export const knownMonths = (rows: ChartRow[], key: string) => rows.filter((r) => r[key] !== null).length

export const BUDGET: Series[] = [
  { key: 'fixed', name: 'Fixas', color: 'var(--series-1)' },
  { key: 'installment', name: 'Parceladas', color: 'var(--series-2)' },
  { key: 'variable', name: 'Variáveis', color: 'var(--series-3)' },
]

export const toBudgetRows = (months: BudgetMonth[]): ChartRow[] =>
  months.map((m) => ({ month: m.month, label: formatMonthShort(m.month), fixed: m.fixed, installment: m.installment, variable: m.variable }))
