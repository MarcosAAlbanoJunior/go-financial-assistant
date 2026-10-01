import { useQuery, keepPreviousData } from '@tanstack/react-query'
import type { Summary, Totals } from './types'

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
