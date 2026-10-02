import { useQuery } from '@tanstack/react-query'
import { request } from '../../shared/api/request'

export interface UncategorizedGroup {
  /** Descrição normalizada: identifica a conta. */
  key: string
  label: string
  count: number
  total: number
  /** Último lançamento (AAAA-MM). */
  last: string
  /** Pix, TED ou transferência: só a pessoa sabe do que se trata, e nunca vai à IA. */
  transfer: boolean
}

export interface Categorize {
  /** Categorias atribuíveis; OTHER significa "manter em Outros". */
  categories: { value: string; label: string }[]
  groups: UncategorizedGroup[]
  ai: { enabled: boolean; blockedReason: string; provider: string; hash: string; bytes: number; context: { contas: { id: string; nome: string; lancamentos: number; total_gasto: number; ultimo_mes: string }[] } }
}

export const useCategorize = () => useQuery({ queryKey: ['categorize'], queryFn: () => request<Categorize>('/api/categorize') })

/** Classifica uma conta: reclassifica as despesas dela em Outros e vale nas próximas sincronizações. */
export const setCategoryRule = (key: string, category: string) =>
  request<{ changed: number }>('/api/categorize/rules', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, category }),
  })

/** Pede à IA categorias para as contas da prévia (o hash garante que é o que a pessoa viu). Nada é gravado. */
export const suggestCategories = (hash: string) =>
  request<{ suggestions: { key: string; category: string }[] }>('/api/categorize/suggest', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ hash }),
  })
