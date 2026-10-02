import { useSearchParams } from 'react-router'
import { currentMonth } from '../../../shared/lib/months'
import { monthFilter, toApiQuery } from './transactions'

export const VIEW_KEYS = ['categoria', 'dia', 'lista'] as const
export type ViewKey = (typeof VIEW_KEYS)[number]

/**
 * Filtros e visão da tela de transações, guardados na URL (dá para compartilhar ou guardar a visão).
 * Qualquer mudança de filtro volta para a primeira página.
 */
export function useTransactionFilters() {
  const [params, setParams] = useSearchParams()
  const now = currentMonth()
  const month = monthFilter(params, now)
  const query = toApiQuery(params, now)
  const requested = params.get('vista')
  const view: ViewKey = VIEW_KEYS.find((k) => k === requested) ?? 'categoria'

  const update = (changes: Record<string, string>) =>
    setParams((prev) => {
      const next = new URLSearchParams(prev)
      for (const [k, v] of Object.entries(changes)) {
        if (v) next.set(k, v)
        else next.delete(k)
      }
      if (!('pagina' in changes)) next.delete('pagina')
      return next
    })

  return { params, now, month, query, view, update }
}
