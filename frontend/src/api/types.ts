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

export type ReviewKind = 'INCREASE' | 'FIXED' | 'ANT' | 'DUPLICATE' | 'NEW'

export interface ReviewRow {
  category: string
  categoryLabel: string
  /** Total de cada mês de `months` (do mais antigo para o mais novo). */
  values: number[]
}

export interface ReviewCandidate {
  kind: ReviewKind
  /** Conta (descrição normalizada) ou, nos aumentos, a categoria. */
  key: string
  label: string
  category: string
  categoryLabel: string
  /** Economia estimada por mês (nas avulsas, o valor de uma vez só). */
  monthly: number
  /** Economia em 12 meses; nula nas avulsas (duplicata e conta nova). */
  annual: number | null
  amount: number
  baseline: number
  count: number
  months: number
  dismissed: boolean
}

export interface Review {
  month: string
  months: string[]
  matrix: ReviewRow[]
  candidates: ReviewCandidate[]
}

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
  history: { month: string; total: number; hit: boolean }[]
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
