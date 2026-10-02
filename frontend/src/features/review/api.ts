import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { request } from '../../shared/api/request'

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

export type SavingStatus = 'PENDING' | 'CONFIRMED' | 'RETURNED'

/** Uma conta marcada como "cancelei", conferida contra o que foi cobrado depois. */
export interface SavingDecision {
  kind: ReviewKind
  key: string
  label: string
  category: string
  categoryLabel: string
  /** Mês em que a pessoa decidiu (AAAA-MM). */
  month: string
  /** Quanto a conta custava por mês. */
  monthly: number
  status: SavingStatus
  /** Meses fechados depois da decisão sem a cobrança. */
  monthsConfirmed: number
  realized: number
  /** Quanto foi cobrado quando voltou (0 se não voltou). */
  returned: number
}

export interface Savings {
  realized: number
  perMonth: number
  perYear: number
  decisions: SavingDecision[]
}

export const useReview = (month: string) =>
  useQuery({
    queryKey: ['review', month],
    queryFn: () => request<Review>(`/api/review?month=${month}`),
    placeholderData: keepPreviousData,
  })

/** Dispensa (ou traz de volta) uma sugestão da revisão. */
export const setDismissal = (kind: ReviewKind, key: string, dismissed: boolean) =>
  request('/api/review-dismissals', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ kind, key, dismissed }),
  })

export const useSavings = () => useQuery({ queryKey: ['savings'], queryFn: () => request<Savings>('/api/savings') })

/** Marca (ou desfaz) "cancelei" numa sugestão do mês; o servidor lê o custo da própria sugestão. */
export const setDecision = (kind: ReviewKind, key: string, month: string, decided: boolean) =>
  request('/api/savings/decisions', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ kind, key, month, decided }),
  })
