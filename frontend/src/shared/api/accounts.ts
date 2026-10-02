import { useQuery } from '@tanstack/react-query'
import { request } from './request'

export interface Account {
  id: string
  type: 'BANK' | 'CREDIT'
  name: string
  last4: string
  balance: number
  creditLimit: number | null
  availableCreditLimit: number | null
  updatedAt: string
}

export const useAccounts = () =>
  useQuery({ queryKey: ['accounts'], queryFn: () => request<Account[]>('/api/accounts') })
