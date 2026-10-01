// Formatos das respostas da API do app Go (backend/internal/infra/http/api.go).

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

export type BreakdownBy = 'category' | 'payment_method' | 'account'

export interface BreakdownItem {
  key: string
  label: string
  total: number
}

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

export interface TransactionPage {
  items: Transaction[]
  total: number
  page: number
  limit: number
}
