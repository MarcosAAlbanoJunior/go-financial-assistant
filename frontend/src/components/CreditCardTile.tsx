import type { CSSProperties } from 'react'
import { CalendarClock, CreditCard, OctagonAlert, TriangleAlert } from 'lucide-react'
import type { BalanceCard, Level } from '../api/types'
import { contrast, dueText, inkOn, LEVEL_LABEL, money, safeHex } from '../lib/balances'
import { formatPercent } from '../lib/format'
import { StaticMoney } from './Money'

const BRANDS: Record<string, string> = { MASTERCARD: 'Mastercard', VISA: 'Visa', ELO: 'Elo', AMEX: 'American Express', HIPERCARD: 'Hipercard' }

function LevelIcon({ level }: { level: Level }) {
  if (level === 'critical') return <OctagonAlert size={14} aria-hidden="true" />
  return <TriangleAlert size={14} aria-hidden="true" />
}

/**
 * Um cartão de crédito, sempre sozinho (cartões não são somados). Fundo em gradiente da cor do banco; o texto
 * é a tinta de maior contraste sobre ela. Atenção e crítico aparecem com ícone e texto.
 */
export function CreditCardTile({ card, color, hidden }: { card: BalanceCard; color: string | null; hidden: boolean }) {
  const brand = safeHex(color)
  const ink = inkOn(brand)
  const dark = ink !== '#ffffff'
  const due = dueText(card)
  const hasLimit = card.limit !== null && card.available !== null && card.usedRatio !== null && card.used !== null
  // Garantia extra: se nem a melhor tinta chega a 4,5:1 sobre a cor de marca, o gradiente vai para o preto.
  const readable = contrast(ink, brand) >= 4.5
  const style = { '--brand': readable ? brand : '#1a1a19', '--on-brand': readable ? ink : '#ffffff' } as CSSProperties

  return (
    <div className={`bal-card ${dark && readable ? 'is-light' : ''}`} style={style}>
      <div className="bal-card-top">
        <span>
          <CreditCard size={16} aria-hidden="true" /> {card.name}
          {card.last4 && ` · final ${card.last4}`}
        </span>
        {card.brand && <span>{BRANDS[card.brand] ?? card.brand}</span>}
      </div>
      <div>
        <p className="bal-card-invoice">
          <StaticMoney value={card.invoice} hidden={hidden} />
        </p>
        <p className="bal-card-sub">saldo devedor · inclui parcelas futuras</p>
        {due && (
          <span className={`bal-pill ${card.dueLevel !== 'ok' ? 'is-alert' : ''}`}>
            {card.dueLevel === 'ok' ? <CalendarClock size={14} aria-hidden="true" /> : <LevelIcon level={card.dueLevel} />} {due}
          </span>
        )}
      </div>
      {hasLimit ? (
        <div>
          <div
            className="bal-meter"
            role="meter"
            aria-label="Limite usado"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={Math.round((card.usedRatio ?? 0) * 100)}
            aria-valuetext={`${formatPercent(card.usedRatio ?? 0)} do limite usado${LEVEL_LABEL[card.usageLevel] ? `, ${LEVEL_LABEL[card.usageLevel]}` : ''}`}
          >
            <i style={{ width: `${(card.usedRatio ?? 0) * 100}%` }} />
          </div>
          <div className="bal-card-row">
            <span className={card.usageLevel !== 'ok' ? 'bal-card-flag' : undefined}>
              {card.usageLevel !== 'ok' && <LevelIcon level={card.usageLevel} />} Usado {formatPercent(card.usedRatio ?? 0)}
              {card.usageLevel !== 'ok' && ` · ${LEVEL_LABEL[card.usageLevel]}`}
            </span>
            <span>
              Disponível <strong>{money(card.available ?? 0, hidden)}</strong> de {money(card.limit ?? 0, hidden)}
            </span>
          </div>
        </div>
      ) : (
        <p className="bal-card-sub">Limite não informado pelo banco</p>
      )}
    </div>
  )
}
