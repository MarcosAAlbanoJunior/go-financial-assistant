import { useSearchParams } from 'react-router'
import { currentMonth, isMonth } from './months'

/** Mês selecionado, guardado na URL (?mes=AAAA-MM). Valor ausente ou inválido vira o mês atual. */
export function useMonth() {
  const [params, setParams] = useSearchParams()
  const now = currentMonth()
  const requested = params.get('mes')
  const month = isMonth(requested) ? requested : now

  const setMonth = (m: string) => {
    if (!isMonth(m)) return
    setParams((prev) => {
      const next = new URLSearchParams(prev)
      next.set('mes', m)
      return next
    })
  }
  return { month, setMonth, now }
}
