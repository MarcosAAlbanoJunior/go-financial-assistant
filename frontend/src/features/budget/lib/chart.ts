import type { BudgetMonth } from '../api'
import { formatMonthShort } from '../../../shared/lib/format'
import type { ChartRow } from '../../../shared/lib/chart'

export const toBudgetRows = (months: BudgetMonth[]): ChartRow[] =>
  months.map((m) => ({ month: m.month, label: formatMonthShort(m.month), fixed: m.fixed, installment: m.installment, variable: m.variable }))
