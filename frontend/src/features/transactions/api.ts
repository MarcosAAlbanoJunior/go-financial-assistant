import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { request } from '../../shared/api/request'

export interface Transaction {
  id: string
  date: string
  description: string
  category: string
  categoryLabel: string
  paymentMethod: string
  paymentMethodLabel: string
  kind: 'EXPENSE' | 'INCOME' | 'TRANSFER'
  transferDirection?: 'IN' | 'OUT'
  type: string
  status: 'PENDING' | 'PAID'
  amount: number
  installmentNumber?: number
  accountId?: string
  accountName?: string
  source: 'MANUAL' | 'OPEN_FINANCE'
}

export interface TransactionGroup {
  /** Categoria (enum) ou data AAAA-MM-DD. */
  key: string
  label: string
  count: number
  expense: number
  income: number
  transfer: number
}

export interface TransactionPage {
  items: Transaction[]
  total: number
  page: number
  limit: number
}

/** query já vem montada e codificada por quem chama (URLSearchParams). */
export const useTransactions = (query: string) =>
  useQuery({
    queryKey: ['transactions', query],
    queryFn: () => request<TransactionPage>(`/api/transactions?${query}`),
    placeholderData: keepPreviousData,
  })

/** query já vem montada e codificada (URLSearchParams); by: "category" ou "day". */
export const useTransactionGroups = (query: string, by: 'category' | 'day') =>
  useQuery({
    queryKey: ['transaction-groups', by, query],
    queryFn: () => request<TransactionGroup[]>(`/api/transactions/groups?${query}&by=${by}`),
    placeholderData: keepPreviousData,
  })
