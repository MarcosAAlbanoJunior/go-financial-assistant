import { money } from '../lib/balances'
import { useCountUp } from '../lib/useCountUp'

interface Props {
  value: number
  hidden: boolean
}

/** Valor em reais que conta até o número ao carregar; oculto, mostra R$ •••• (e não anima). */
export function Money({ value, hidden }: Props) {
  const shown = useCountUp(value)
  return <span className="bal-money">{money(hidden ? value : shown, hidden)}</span>
}

/** Sem animação, para valores pequenos e secundários. */
export function StaticMoney({ value, hidden }: Props) {
  return <span className="bal-money">{money(value, hidden)}</span>
}
