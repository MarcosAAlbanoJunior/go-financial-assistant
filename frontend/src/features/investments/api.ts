import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { request } from '../../shared/api/request'
import type { BreakdownItem } from '../../shared/api/totals'

export interface InvestmentMonth {
  month: string
  applied: number
  redeemed: number
  /** Aplicado - resgatado, somado desde o primeiro lançamento (não só dentro da janela). */
  cumulative: number
}

export interface Position {
  id: string
  type: string
  typeLabel: string
  subtype: string
  name: string
  /** Saldo líquido atual. */
  balance: number
  /** Valor bruto. */
  amount: number
  updatedAt: string
}

export interface Portfolio {
  total: number
  positions: Position[]
  byType: BreakdownItem[]
}

export interface PortfolioMonth {
  month: string
  /** Saldo total ao fim do mês; null quando não há como saber. */
  balance: number | null
  /** true nos meses anteriores à primeira sincronização: valor reconstruído pelas movimentações, não o saldo exato. */
  estimated: boolean
}

export const useInvestments = (from: string, to: string) =>
  useQuery({
    queryKey: ['investments', from, to],
    queryFn: () => request<InvestmentMonth[]>(`/api/investments?from=${from}&to=${to}`),
    placeholderData: keepPreviousData,
  })

export const usePortfolio = () => useQuery({ queryKey: ['portfolio'], queryFn: () => request<Portfolio>('/api/portfolio') })

export const usePortfolioHistory = (from: string, to: string) =>
  useQuery({
    queryKey: ['portfolio-history', from, to],
    queryFn: () => request<PortfolioMonth[]>(`/api/portfolio/history?from=${from}&to=${to}`),
    placeholderData: keepPreviousData,
  })
