import { useQuery } from '@tanstack/react-query'
import { request } from '../../shared/api/request'

export type Level = 'ok' | 'warning' | 'critical'

export interface BalanceAccount {
  id: string
  name: string
  last4: string
  balance: number
  /** Aplicado automaticamente pelo banco; fora do total em conta. */
  autoInvested: number | null
}

export interface BalanceCard {
  id: string
  name: string
  brand: string
  last4: string
  /** Valor devido informado pelo banco. */
  invoice: number
  limit: number | null
  available: number | null
  used: number | null
  usedRatio: number | null
  usageLevel: Level
  closeDate: string | null
  dueDate: string | null
  daysToDue: number | null
  dueLevel: Level
  minimumPayment: number | null
}

export interface BalanceInstitution {
  id: string
  name: string
  /** Cor de marca (#rrggbb) ou null. */
  color: string | null
  /** Endereço do logo na própria API, ou null (a tela usa o monograma). */
  logo: string | null
  updatedAt: string
  stale: boolean
  /** Soma das contas correntes; null se o banco só tem cartão. */
  total: number | null
  shareOfTotal: number
  accounts: BalanceAccount[]
  cards: BalanceCard[]
}

export interface Balances {
  asOf: string | null
  totalInAccount: number | null
  openInvoices: number
  institutions: BalanceInstitution[]
}

export const useBalances = () => useQuery({ queryKey: ['balances'], queryFn: () => request<Balances>('/api/balances') })

