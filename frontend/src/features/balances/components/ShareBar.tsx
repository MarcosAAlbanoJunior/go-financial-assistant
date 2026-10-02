import { useEffect, useState } from 'react'
import { TriangleAlert } from 'lucide-react'
import { shareLabel, sliceColors, sliced } from '../lib/balances'
import { formatPercent } from '../../../shared/lib/format'
import { BankMark } from './BankMark'
import { StaticMoney } from './Money'
import type { BalanceInstitution } from '../api'

/**
 * Participação de cada banco no total em conta: uma barra com vão de 2 px e a legenda com logo, nome,
 * valor e percentual. A legenda diz tudo; a cor só ajuda. Banco negativo mostra o valor, sem fatia.
 */
export function ShareBar({ institutions, hidden }: { institutions: BalanceInstitution[]; hidden: boolean }) {
  const [grown, setGrown] = useState(false)
  // A barra cresce ao carregar (a transição some com prefers-reduced-motion, no CSS).
  useEffect(() => {
    const id = requestAnimationFrame(() => setGrown(true))
    return () => cancelAnimationFrame(id)
  }, [])

  const withBank = institutions.filter((i) => i.total !== null)
  const parts = sliced(institutions)
  const colors = sliceColors(parts.map((i) => i.color))
  if (withBank.length === 0) return null

  return (
    <>
      {parts.length > 0 && (
        <div className="bal-share" role="img" aria-label={shareLabel(institutions)}>
          {parts.map((i, n) => (
            <span key={i.id} style={{ width: grown ? `${i.shareOfTotal * 100}%` : '0%', background: colors[n].css }} />
          ))}
        </div>
      )}
      <ul className="bal-legend">
        {withBank.map((i) => (
          <li key={i.id}>
            <BankMark name={i.name} color={i.color} logo={i.logo} size="sm" />
            <span>{i.name}</span>
            <strong>
              <StaticMoney value={i.total ?? 0} hidden={hidden} />
            </strong>
            {i.shareOfTotal > 0 && <span className="bal-muted">{formatPercent(i.shareOfTotal)}</span>}
            {(i.total ?? 0) < 0 && (
              <span className="bal-flag bal-flag-critical">
                <TriangleAlert size={14} aria-hidden="true" /> saldo negativo
              </span>
            )}
          </li>
        ))}
      </ul>
    </>
  )
}
