import { useQuery, keepPreviousData } from '@tanstack/react-query'
import type { Account, Budget, CoachPreview, CoachResult, Goals, GoalInput, Projection, Review, ReviewKind, InvestmentMonth, Portfolio, PortfolioMonth, BreakdownBy, BreakdownItem, Summary, Totals, TransactionGroup, TransactionPage } from './types'

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  // Mesma origem: o cookie de sessão (HttpOnly) vai sozinho, e o front nunca vê a senha nem o cookie.
  const res = await fetch(path, { credentials: 'same-origin', ...init })
  const body = await res.json().catch(() => null)
  if (!res.ok) throw new ApiError(res.status, body?.error ?? 'erro inesperado')
  return body as T
}

const postJSON = <T>(path: string, data?: unknown) =>
  request<T>(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data ?? {}),
  })

export const login = (password: string) => postJSON('/api/login', { password })
export const logout = () => postJSON('/api/logout')

// Sem retry em 401/403: repetir não muda o resultado e só atrasa o redirecionamento ao login.
export const useMe = () =>
  useQuery({ queryKey: ['me'], queryFn: () => request('/api/me'), retry: false, staleTime: 60_000 })

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

export const useAccounts = () =>
  useQuery({ queryKey: ['accounts'], queryFn: () => request<Account[]>('/api/accounts') })

/** query já vem montada e codificada por quem chama (URLSearchParams). */
export const useTransactions = (query: string) =>
  useQuery({
    queryKey: ['transactions', query],
    queryFn: () => request<TransactionPage>(`/api/transactions?${query}`),
    placeholderData: keepPreviousData,
  })

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

/** query já vem montada e codificada (URLSearchParams); by: "category" ou "day". */
export const useTransactionGroups = (query: string, by: 'category' | 'day') =>
  useQuery({
    queryKey: ['transaction-groups', by, query],
    queryFn: () => request<TransactionGroup[]>(`/api/transactions/groups?${query}&by=${by}`),
    placeholderData: keepPreviousData,
  })

export const useBudget = (month: string) =>
  useQuery({
    queryKey: ['budget', month],
    queryFn: () => request<Budget>(`/api/budget?month=${month}`),
    placeholderData: keepPreviousData,
  })

/** Corrige à mão se uma conta é fixa ou variável; "AUTO" volta à detecção automática. */
export const setExpenseRule = (key: string, cls: 'FIXED' | 'VARIABLE' | 'AUTO') =>
  request('/api/expense-rules', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, class: cls }),
  })

export const useProjection = (months: number) =>
  useQuery({
    queryKey: ['projection', months],
    queryFn: () => request<Projection>(`/api/projection?months=${months}`),
    placeholderData: keepPreviousData,
  })

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

export const useGoals = () => useQuery({ queryKey: ['goals'], queryFn: () => request<Goals>('/api/goals') })

export const createGoal = (input: GoalInput) =>
  request('/api/goals', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })

export const deleteGoal = (id: string) => request(`/api/goals/${encodeURIComponent(id)}`, { method: 'DELETE' })

/** Só lê: mostra o que o Coach enviaria, sem enviar nada. */
export const useCoachPreview = (month: string) =>
  useQuery({
    queryKey: ['coach-preview', month],
    queryFn: () => request<CoachPreview>(`/api/coach/preview?month=${month}`),
  })

/** Envia ao Gemini o contexto da prévia (o hash garante que é o que a pessoa viu). */
export const analyzeCoach = (month: string, hash: string) =>
  request<CoachResult>('/api/coach/analyze', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ month, hash }),
  })
