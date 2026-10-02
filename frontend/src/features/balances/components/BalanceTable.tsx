import { dueText, money } from '../lib/balances'
import { formatPercent } from '../../../shared/lib/format'
import type { Balances } from '../api'

/** Os mesmos valores do painel em tabela: visão alternativa para leitor de tela e para conferir números. */
export function BalanceTable({ data, hidden }: { data: Balances; hidden: boolean }) {
  return (
    <details className="card bal-table">
      <summary>Ver como tabela</summary>
      <div className="table-scroll">
        <table className="data">
          <caption className="sr-only">Saldos por banco, contas correntes e cartões de crédito</caption>
          <thead>
            <tr>
              <th scope="col">Banco</th>
              <th scope="col">Conta ou cartão</th>
              <th scope="col">Saldo ou saldo devedor</th>
              <th scope="col">Limite usado</th>
              <th scope="col">Vencimento</th>
            </tr>
          </thead>
          <tbody>
            {data.institutions.flatMap((i) => [
              ...i.accounts.map((a) => (
                <tr key={a.id}>
                  <th scope="row">{i.name}</th>
                  <td>
                    {a.name}
                    {a.last4 && ` · final ${a.last4}`}
                  </td>
                  <td>{money(a.balance, hidden)}</td>
                  <td>—</td>
                  <td>—</td>
                </tr>
              )),
              ...i.cards.map((c) => (
                <tr key={c.id}>
                  <th scope="row">{i.name}</th>
                  <td>
                    {c.name}
                    {c.last4 && ` · final ${c.last4}`} (cartão)
                  </td>
                  <td>{money(c.invoice, hidden)}</td>
                  <td>{c.usedRatio === null ? 'não informado' : formatPercent(c.usedRatio)}</td>
                  <td>{dueText(c) ?? '—'}</td>
                </tr>
              )),
            ])}
          </tbody>
        </table>
      </div>
    </details>
  )
}
