import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { request } from './request'

export interface Totals {
  month: string
  income: number
  expense: number
  applied: number
  redeemed: number
}

export interface Summary {
  month: string
  current: Totals
  previous: Totals
  /** Soma das contas correntes; null quando não há conta sincronizada. */
  bankBalance: number | null
}

export type BreakdownBy = 'category' | 'payment_method' | 'account'

export interface BreakdownItem {
  key: string
  label: string
  total: number
}

// placeholderData mantém o gráfico anterior na tela (esmaecido) enquanto o novo carrega.
export const useSummary = (month: string) =>
  useQuery({
    queryKey: ['summary', month],
    queryFn: () => request<Summary>(`/api/summary?month=${month}`),
    placeholderData: keepPreviousData,
  })

export const useTimeseries = (from: string, to: string) =>
  useQuery({
    queryKey: ['timeseries', from, to],
    queryFn: () => request<Totals[]>(`/api/timeseries?from=${from}&to=${to}`),
    placeholderData: keepPreviousData,
  })

export const useBreakdown = (month: string, by: BreakdownBy) =>
  useQuery({
    queryKey: ['breakdown', month, by],
    queryFn: () => request<BreakdownItem[]>(`/api/breakdown?month=${month}&by=${by}`),
    placeholderData: keepPreviousData,
  })
