import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { request } from '../../shared/api/request'

export type ExpenseClass = 'FIXED' | 'INSTALLMENT' | 'VARIABLE'

export interface BudgetMonth {
  month: string
  fixed: number
  installment: number
  variable: number
}

export interface BudgetItem {
  /** Descrição normalizada: identifica a mesma conta em meses diferentes. */
  key: string
  label: string
  category: string
  categoryLabel: string
  class: ExpenseClass
  /** A classe veio de uma correção manual, não da detecção automática. */
  manual: boolean
  total: number
  count: number
  day: number
  paid: boolean
  /** Em quantos dos últimos 12 meses a conta aparece. */
  months: number
}

export interface Budget {
  month: string
  series: BudgetMonth[]
  items: BudgetItem[]
}

export const useBudget = (month: string) =>
  useQuery({
    queryKey: ['budget', month],
    queryFn: () => request<Budget>(`/api/budget?month=${month}`),
    placeholderData: keepPreviousData,
  })

/** Corrige à mão se uma conta é fixa ou variável; "AUTO" volta à detecção automática. */
export const setExpenseRule = (key: string, cls: 'FIXED' | 'VARIABLE' | 'AUTO') =>
  request('/api/expense-rules', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, class: cls }),
  })
