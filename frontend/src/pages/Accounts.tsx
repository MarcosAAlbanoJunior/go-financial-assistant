import { useAccounts } from '../api/client'
import type { Account } from '../api/types'
import { QueryState } from '../components/QueryState'
import { formatBRL, formatPercent } from '../lib/format'

export default function Accounts() {
  const query = useAccounts()
  return (
    <>
      <h1 className="page-title">Contas e cartões</h1>
      <QueryState query={query}>
        {(accounts) =>
          accounts.length === 0 ? (
            <p className="state">Nenhuma conta sincronizada. Configure o Open Finance (veja o README).</p>
          ) : (
            <div className="grid-2 section-gap">
              {accounts.map((a) => (
                <AccountCard key={a.id} account={a} />
              ))}
            </div>
          )
        }
      </QueryState>
    </>
  )
}

function AccountCard({ account: a }: { account: Account }) {
  const updated = new Date(a.updatedAt).toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' })
  return (
    <article className="card">
      <p className="tile-label">{a.type === 'BANK' ? 'Conta corrente' : 'Cartão de crédito'}</p>
      <h2 className="chart-title">
        {a.name} {a.last4 && <span className="tile-note">final {a.last4}</span>}
      </h2>
      {a.type === 'BANK' ? (
        <>
          <p className="tile-value">{formatBRL(a.balance)}</p>
          <p className="tile-note">Saldo informado pelo banco</p>
        </>
      ) : (
        <CreditLimit limit={a.creditLimit} available={a.availableCreditLimit} />
      )}
      <p className="tile-note">Atualizado em {updated}</p>
    </article>
  )
}

function CreditLimit({ limit, available }: { limit: number | null; available: number | null }) {
  if (limit === null || available === null || limit <= 0) {
    return <p className="tile-note">Limite não informado pelo banco</p>
  }
  const used = Math.max(0, limit - available)
  const ratio = Math.min(1, used / limit)
  // A cor reforça a severidade, mas o percentual escrito é quem informa.
  const level = ratio >= 0.9 ? 'critical' : ratio >= 0.7 ? 'warning' : 'ok'

  return (
    <>
      <p className="tile-value">{formatBRL(available)}</p>
      <p className="tile-note">disponível de {formatBRL(limit)}</p>
      <div
        className="meter"
        role="meter"
        aria-label="Limite usado"
        aria-valuemin={0}
        aria-valuemax={limit}
        aria-valuenow={used}
        aria-valuetext={`${formatBRL(used)} usados de ${formatBRL(limit)}`}
      >
        <div className={`meter-fill meter-${level}`} style={{ width: `${ratio * 100}%` }} />
      </div>
      <p className="tile-note">
        {formatPercent(ratio)} do limite usado ({formatBRL(used)})
      </p>
    </>
  )
}
