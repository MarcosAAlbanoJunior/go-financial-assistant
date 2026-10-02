import type { Balances } from '../api/types'
import { freshnessText } from '../lib/balances'
import { Money, StaticMoney } from './Money'
import { ShareBar } from './ShareBar'

export function BalanceHero({ data, hidden, now }: { data: Balances; hidden: boolean; now: Date }) {
  const banks = data.institutions.filter((i) => i.total !== null).length
  const stale = data.institutions.some((i) => i.stale)

  return (
    <section className="bal-hero" aria-labelledby="bal-total-label">
      <p className="bal-hero-label" id="bal-total-label">
        Total em conta
      </p>
      <p className="bal-hero-value">{data.totalInAccount === null ? '—' : <Money value={data.totalInAccount} hidden={hidden} />}</p>
      <p className="tile-note">
        {data.totalInAccount === null
          ? 'Nenhuma conta corrente sincronizada'
          : `${banks} ${banks === 1 ? 'banco' : 'bancos'}`}
        {data.asOf && ` · atualizado ${freshnessText(data.asOf, now)}`}
        {stale && ' · há bancos desatualizados'}
      </p>
      <ShareBar institutions={data.institutions} hidden={hidden} />
      {data.openInvoices > 0 && (
        <p className="bal-aside">
          Saldo devedor dos cartões, à parte (não subtraído do total): <strong><StaticMoney value={data.openInvoices} hidden={hidden} /></strong>
        </p>
      )}
    </section>
  )
}
