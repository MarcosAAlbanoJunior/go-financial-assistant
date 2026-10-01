import type { Transaction } from '../api/types'
import { formatBRL } from './format'
import { currentMonth, isMonth } from './months'

export const FILTER_KEYS = ['tipo', 'categoria', 'forma', 'conta', 'q'] as const

/** "todos" na URL desliga o filtro de mês; ausente ou inválido vale o mês atual. */
export function monthFilter(params: URLSearchParams, now = currentMonth()): string | null {
  const raw = params.get('mes')
  if (raw === 'todos') return null
  return isMonth(raw) ? raw : now
}

/** Converte os parâmetros da URL (pt-BR) na query da API, descartando o que está vazio. */
export function toApiQuery(params: URLSearchParams, now = currentMonth()): string {
  const out = new URLSearchParams()
  const month = monthFilter(params, now)
  if (month) out.set('month', month)

  const map: Record<string, string> = { tipo: 'kind', categoria: 'category', forma: 'payment_method', conta: 'account', q: 'q' }
  for (const key of FILTER_KEYS) {
    const value = params.get(key)?.trim()
    if (value) out.set(map[key], value)
  }
  const page = Number(params.get('pagina'))
  if (Number.isInteger(page) && page > 1) out.set('page', String(page))
  return out.toString()
}

/** Valor com sinal e o que ele significa. Investimento não é gasto nem ganho: fica neutro. */
export function describeAmount(t: Pick<Transaction, 'kind' | 'transferDirection' | 'amount'>) {
  const value = formatBRL(t.amount)
  switch (t.kind) {
    case 'INCOME':
      return { text: `+${value}`, tone: 'in' as const, caption: 'Receita' }
    case 'TRANSFER':
      return { text: value, tone: 'neutral' as const, caption: t.transferDirection === 'IN' ? 'Resgate' : 'Aplicação' }
    default:
      return { text: `-${value}`, tone: 'out' as const, caption: 'Despesa' }
  }
}

/** "2026-09-03" → "qui, 03/09" (dia da semana calculado em UTC, sem deslocar o dia). */
export function formatDayLabel(iso: string): string {
  const [y, m, d] = iso.split('-').map(Number)
  const weekday = new Date(Date.UTC(y, m - 1, d)).toLocaleDateString('pt-BR', { weekday: 'short', timeZone: 'UTC' }).replace('.', '')
  return `${weekday}, ${String(d).padStart(2, '0')}/${String(m).padStart(2, '0')}`
}

/** Parâmetros de um filtro extra (categoria ou dia) sobre a query atual, para listar o detalhe de um grupo. */
export function withGroupFilter(query: string, key: 'category' | 'day', value: string): string {
  const q = new URLSearchParams(query)
  q.set(key, value)
  q.delete('page')
  q.set('limit', '100')
  if (key === 'day') q.delete('month')
  return q.toString()
}

/** "2026-09-03" → "03/09/2026", sem passar por Date (evita deslocamento de fuso). */
export function formatDay(iso: string): string {
  const [y, m, d] = iso.split('-')
  return `${d}/${m}/${y}`
}
