import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router'
import { useAccounts, useTransactions } from '../api/client'
import { QueryState } from '../components/QueryState'
import { formatMonthTitle } from '../lib/format'
import { CATEGORIES, KINDS, PAYMENT_METHODS } from '../lib/labels'
import { currentMonth, shiftMonth } from '../lib/months'
import { describeAmount, formatDay, monthFilter, toApiQuery } from '../lib/transactions'

export default function Transactions() {
  const [params, setParams] = useSearchParams()
  const now = currentMonth()
  const month = monthFilter(params, now)
  const query = toApiQuery(params, now)
  const result = useTransactions(query)
  const accounts = useAccounts()

  // Qualquer mudança de filtro volta para a primeira página.
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

  const select = (key: string, label: string, options: { value: string; label: string }[]) => (
    <label className="field">
      <span className="sr-only">{label}</span>
      <select value={params.get(key) ?? ''} onChange={(e) => update({ [key]: e.target.value })}>
        <option value="">{label}</option>
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  )

  return (
    <>
      <h1 className="page-title">Transações{month ? ` de ${formatMonthTitle(month)}` : ''}</h1>

      <div className="filters">
        <button type="button" className="btn" aria-label="Mês anterior" disabled={!month} onClick={() => month && update({ mes: shiftMonth(month, -1) })}>
          ←
        </button>
        <input
          type="month"
          aria-label="Mês"
          value={month ?? ''}
          max={now}
          disabled={!month}
          onChange={(e) => e.target.value && update({ mes: e.target.value })}
        />
        <button type="button" className="btn" aria-label="Próximo mês" disabled={!month || month >= now} onClick={() => month && update({ mes: shiftMonth(month, 1) })}>
          →
        </button>
        <label className="check">
          <input type="checkbox" checked={!month} onChange={(e) => update({ mes: e.target.checked ? 'todos' : now })} />
          Todos os meses
        </label>
      </div>
      <div className="filters">
        {select('tipo', 'Tipo', KINDS)}
        {select('categoria', 'Categoria', CATEGORIES)}
        {select('forma', 'Forma de pagamento', PAYMENT_METHODS)}
        {select('conta', 'Conta ou cartão', (accounts.data ?? []).map((a) => ({ value: a.id, label: `${a.name} ${a.last4}`.trim() })))}
        <SearchBox value={params.get('q') ?? ''} onCommit={(q) => update({ q })} />
      </div>

      <QueryState query={result}>
        {(page) => (
          <section className={`card chart-body ${result.isPlaceholderData ? 'is-stale' : ''}`} aria-label="Lista de transações">
            {page.items.length === 0 ? (
              <p className="state">Nenhuma transação encontrada.</p>
            ) : (
              <div className="table-scroll">
                <table className="data tx">
                  <thead>
                    <tr>
                      <th scope="col">Data</th>
                      <th scope="col">Descrição</th>
                      <th scope="col">Categoria</th>
                      <th scope="col">Pagamento</th>
                      <th scope="col">Conta</th>
                      <th scope="col">Valor</th>
                    </tr>
                  </thead>
                  <tbody>
                    {page.items.map((t) => {
                      const a = describeAmount(t)
                      return (
                        <tr key={t.id}>
                          <td>{formatDay(t.date)}</td>
                          <td className="left">
                            {t.description || '—'}
                            {t.status === 'PENDING' && <span className="badge">Pendente</span>}
                            {t.source === 'MANUAL' && <span className="badge">Manual</span>}
                          </td>
                          <td className="left">{t.categoryLabel}</td>
                          <td className="left">{t.paymentMethodLabel}</td>
                          <td className="left">{t.accountName || '—'}</td>
                          <td className={`amount amount-${a.tone}`}>
                            {a.text}
                            <span className="amount-caption">{a.caption}</span>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            )}
            <Pager page={page.page} limit={page.limit} total={page.total} onPage={(n) => update({ pagina: n > 1 ? String(n) : '' })} />
          </section>
        )}
      </QueryState>
    </>
  )
}

/** Busca com espera: só consulta a API 300ms depois da última tecla. */
function SearchBox({ value, onCommit }: { value: string; onCommit: (q: string) => void }) {
  const [text, setText] = useState(value)
  // Se a URL mudar por fora (botão voltar), o campo acompanha.
  const [seen, setSeen] = useState(value)
  if (value !== seen) {
    setSeen(value)
    setText(value)
  }
  useEffect(() => {
    if (text === value) return
    const id = setTimeout(() => onCommit(text), 300)
    return () => clearTimeout(id)
  }, [text, value, onCommit])

  return (
    <label className="field">
      <span className="sr-only">Buscar na descrição</span>
      <input type="search" placeholder="Buscar na descrição" maxLength={100} value={text} onChange={(e) => setText(e.target.value)} />
    </label>
  )
}

function Pager({ page, limit, total, onPage }: { page: number; limit: number; total: number; onPage: (n: number) => void }) {
  const pages = Math.max(1, Math.ceil(total / limit))
  return (
    <div className="pager">
      <span>
        {total} {total === 1 ? 'transação' : 'transações'}
      </span>
      <div>
        <button type="button" className="btn" disabled={page <= 1} onClick={() => onPage(page - 1)}>
          Anterior
        </button>
        <span className="pager-pos">
          {page} de {pages}
        </span>
        <button type="button" className="btn" disabled={page >= pages} onClick={() => onPage(page + 1)}>
          Próxima
        </button>
      </div>
    </div>
  )
}
