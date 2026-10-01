import type { Totals } from '../api/types'
import { formatMonthShort } from './format'

export interface MonthRow {
  month: string
  label: string
  income: number
  expense: number
}

export const toMonthRows = (totals: Totals[]): MonthRow[] =>
  totals.map((t) => ({ month: t.month, label: formatMonthShort(t.month), income: t.income, expense: t.expense }))
