import { useQuery } from '@tanstack/react-query'
import { request } from '../../shared/api/request'

export type GoalKind = 'SAVE' | 'CUT' | 'RESERVE'

export interface Goal {
  id: string
  kind: GoalKind
  name: string
  /** SAVE: mês-alvo (AAAA-MM). */
  targetDate: string | null
  /** CUT */
  category: string
  categoryLabel: string
  cutPercent: number
  /** CUT: média mensal da categoria antes da meta. */
  baseline: number
  /** RESERVE */
  reserveMonths: number
  /** SAVE e RESERVE: patrimônio; CUT: gasto da categoria no mês atual. */
  current: number
  /** SAVE: valor; RESERVE: meses x fixas; CUT: teto mensal. */
  target: number
  done: boolean
  monthsLeft: number
  perMonth: number
  /** Sobra média projetada até a data; nulo sem histórico para projetar. */
  surplus: number | null
  fits: boolean | null
  /** Quantos meses de despesas fixas o patrimônio cobre. */
  coverage: number
  /** Do mês da criação até o atual; o último é o mês ainda aberto. */
  history: { month: string; total: number; hit: boolean }[]
  /** Quanto o mês fecha no ritmo atual; nulo nos primeiros dias do mês. */
  projected: number | null
  dayOfMonth: number
  daysInMonth: number
}

export interface Goals {
  wealth: number
  goals: Goal[]
}

export interface GoalInput {
  kind: GoalKind
  name: string
  targetAmount?: number
  targetDate?: string
  category?: string
  cutPercent?: number
  reserveMonths?: number
}

export const useGoals = () => useQuery({ queryKey: ['goals'], queryFn: () => request<Goals>('/api/goals') })

export const createGoal = (input: GoalInput) =>
  request('/api/goals', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })

export const deleteGoal = (id: string) => request(`/api/goals/${encodeURIComponent(id)}`, { method: 'DELETE' })
