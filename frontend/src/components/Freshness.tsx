import { TriangleAlert } from 'lucide-react'
import { freshnessText } from '../lib/balances'

/** Horário da última atualização; "desatualizado" com ícone e texto (nunca só cor). */
export function Freshness({ updatedAt, stale, now }: { updatedAt: string; stale: boolean; now: Date }) {
  const text = freshnessText(updatedAt, now)
  if (!stale) return <span className="bal-fresh">{text}</span>
  return (
    <span className="bal-fresh bal-fresh-stale">
      <TriangleAlert size={14} aria-hidden="true" /> desatualizado · {text}
    </span>
  )
}
