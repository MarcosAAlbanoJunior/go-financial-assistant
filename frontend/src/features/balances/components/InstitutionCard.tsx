import type { CSSProperties } from 'react'
import { inkOn, safeHex } from '../lib/balances'
import { BankMark } from './BankMark'
import { CreditCardTile } from './CreditCardTile'
import { Freshness } from './Freshness'
import { Money, StaticMoney } from './Money'
import type { BalanceInstitution } from '../api'

/** O banco como cartão principal: logo, nome e atualização; dentro dele, as contas e os cartões daquele banco. */
export function InstitutionCard({ institution: i, hidden, now }: { institution: BalanceInstitution; hidden: boolean; now: Date }) {
  const brand = safeHex(i.color)
  const style = { '--brand': brand, '--on-brand': inkOn(brand) } as CSSProperties

  return (
    <article className="bal-bank" style={style} aria-label={i.name}>
      <header className="bal-bank-head">
        <BankMark name={i.name} color={i.color} logo={i.logo} />
        <div className="bal-bank-id">
          <h2 className="bal-bank-name">{i.name}</h2>
          <Freshness updatedAt={i.updatedAt} stale={i.stale} now={now} />
        </div>
      </header>

      {i.accounts.map((a) => (
        <div key={a.id}>
          <p className="bal-acc-label">
            {a.name}
            {a.last4 && ` · final ${a.last4}`}
          </p>
          <p className="bal-balance">
            <Money value={a.balance} hidden={hidden} />
          </p>
          {a.autoInvested !== null && a.autoInvested > 0 && (
            <p className="tile-note">
              Aplicado automaticamente <StaticMoney value={a.autoInvested} hidden={hidden} /> (fora do total, já está no Patrimônio)
            </p>
          )}
        </div>
      ))}

      {i.cards.length > 0 && (
        <div className="bal-cards">
          {i.cards.map((c) => (
            <CreditCardTile key={c.id} card={c} color={i.color} hidden={hidden} />
          ))}
        </div>
      )}
      {i.cards.length === 0 && i.accounts.length > 0 && <p className="tile-note bal-nocard">Sem cartão de crédito neste banco</p>}
    </article>
  )
}
