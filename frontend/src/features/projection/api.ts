import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { request } from '../../shared/api/request'

export interface Projection {
  assumptions: {
    income: number
    /** Fontes de renda recorrentes e quanto cada uma rende em um mês comum. */
    incomeSources: { label: string; monthly: number }[]
    fixed: number
    variable: number
    basedOn: number
  }
  months: { month: string; fixed: number; installment: number; variable: number }[]
}

export const useProjection = (months: number) =>
  useQuery({
    queryKey: ['projection', months],
    queryFn: () => request<Projection>(`/api/projection?months=${months}`),
    placeholderData: keepPreviousData,
  })
