import { formatMonthShort } from './format'
import type { Totals } from '../api/totals'

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

export const toMonthRows = (totals: Totals[]): ChartRow[] =>
  totals.map((t) => ({ month: t.month, label: formatMonthShort(t.month), income: t.income, expense: t.expense }))

/** Quantos meses têm saldo registrado (o histórico começa na primeira sincronização). */
export const knownMonths = (rows: ChartRow[], key: string) => rows.filter((r) => r[key] !== null).length

export const BUDGET: Series[] = [
  { key: 'fixed', name: 'Fixas', color: 'var(--series-1)' },
  { key: 'installment', name: 'Parceladas', color: 'var(--series-2)' },
  { key: 'variable', name: 'Variáveis', color: 'var(--series-3)' },
]

