import { CalendarDays, LayoutGrid, List } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router'
import { useAccounts, useTransactionGroups, useTransactions } from '../api/client'
import type { TransactionGroup } from '../api/types'
import { CategoryChip } from '../components/CategoryChip'
import { GroupCard } from '../components/GroupCard'
import { QueryState } from '../components/QueryState'
import { StatTile } from '../components/StatTile'
import { TransactionTable } from '../components/TransactionTable'
import { categoryVisual } from '../lib/categoryVisual'
import { formatBRL, formatMonthTitle } from '../lib/format'
import { CATEGORIES, KINDS, PAYMENT_METHODS } from '../lib/labels'
import { currentMonth, shiftMonth } from '../lib/months'
import { formatDayLabel, monthFilter, toApiQuery, withGroupFilter } from '../lib/transactions'

export default function Transactions() {
  const [params, setParams] = useSearchParams()
  const now = currentMonth()
  const month = monthFilter(params, now)
  const query = toApiQuery(params, now)
  const view = VIEWS.some((v) => v.key === params.get('vista')) ? (params.get('vista') as ViewKey) : 'categoria'
  const accounts = useAccounts()
  // Os totais do topo vêm dos grupos por categoria, agregados no SQL, qualquer que seja a visão.
  const categories = useTransactionGroups(query, 'category')

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

      <Summary groups={categories.data} />

      <div className="view-toggle" role="group" aria-label="Forma de exibir">
        {VIEWS.map(({ key, label, Icon }) => (
          <button key={key} type="button" className="btn" aria-pressed={view === key} onClick={() => update({ vista: key === 'categoria' ? '' : key })}>
            <Icon size={16} aria-hidden="true" /> {label}
          </button>
        ))}
      </div>

      {view === 'lista' && <ListView query={query} onPage={(n) => update({ pagina: n > 1 ? String(n) : '' })} />}
      {view === 'categoria' && (
        <QueryState query={categories}>
          {(groups) => <CategoryCards groups={groups} query={query} stale={categories.isPlaceholderData} />}
        </QueryState>
      )}
      {view === 'dia' && <DayCards query={query} />}
    </>
  )
}

const VIEWS = [
  { key: 'categoria', label: 'Por categoria', Icon: LayoutGrid },
  { key: 'dia', label: 'Por dia', Icon: CalendarDays },
  { key: 'lista', label: 'Lista', Icon: List },
] as const
type ViewKey = (typeof VIEWS)[number]['key']

/** Totais do que está filtrado: despesas, receitas e investimentos. */
function Summary({ groups }: { groups: TransactionGroup[] | undefined }) {
  if (!groups) return null
  const total = groups.reduce(
    (acc, g) => ({ expense: acc.expense + g.expense, income: acc.income + g.income, transfer: acc.transfer + g.transfer, count: acc.count + g.count }),
    { expense: 0, income: 0, transfer: 0, count: 0 },
  )
  return (
    <div className="tiles section-gap">
      <StatTile label="Despesas" value={formatBRL(total.expense)} />
      <StatTile label="Receitas" value={formatBRL(total.income)} />
      <StatTile label="Investimentos" value={formatBRL(total.transfer)} note="Aplicações e resgates" />
      <StatTile label="Lançamentos" value={String(total.count)} />
    </div>
  )
}

function CategoryCards({ groups, query, stale }: { groups: TransactionGroup[]; query: string; stale: boolean }) {
  if (groups.length === 0) return <p className="state">Nenhuma transação encontrada.</p>
  const max = Math.max(...groups.map((g) => Math.max(g.expense, g.income, g.transfer)), 1)
  return (
    <div className={`group-grid chart-body ${stale ? 'is-stale' : ''}`}>
      {groups.map((g) => {
        const { color } = categoryVisual(g.key)
        return (
          <GroupCard
            key={g.key}
            color={color}
            icon={<CategoryChip category={g.key} size={20} />}
            title={g.label}
            subtitle={`${g.count} ${g.count === 1 ? 'lançamento' : 'lançamentos'}`}
            {...amounts(g)}
            share={Math.max(g.expense, g.income, g.transfer) / max}
          >
            {() => <GroupDetails query={withGroupFilter(query, 'category', g.key)} hideCategory />}
          </GroupCard>
        )
      })}
    </div>
  )
}

function DayCards({ query }: { query: string }) {
  const days = useTransactionGroups(query, 'day')
  return (
    <QueryState query={days}>
      {(groups) => {
        if (groups.length === 0) return <p className="state">Nenhuma transação encontrada.</p>
        const max = Math.max(...groups.map((g) => Math.max(g.expense, g.income, g.transfer)), 1)
        return (
          <div className={`group-grid chart-body ${days.isPlaceholderData ? 'is-stale' : ''}`}>
            {groups.map((g) => (
              <GroupCard
                key={g.key}
                color="var(--accent)"
                icon={<span className="chip chip-day">{g.key.slice(8)}</span>}
                title={formatDayLabel(g.key)}
                subtitle={`${g.count} ${g.count === 1 ? 'lançamento' : 'lançamentos'}`}
                {...amounts(g)}
                share={Math.max(g.expense, g.income, g.transfer) / max}
              >
                {() => <GroupDetails query={withGroupFilter(query, 'day', g.key)} />}
              </GroupCard>
            ))}
          </div>
        )
      }}
    </QueryState>
  )
}

/** Valor do cabeçalho: despesa quando há; receitas e investimentos viram linhas menores. */
function amounts(g: TransactionGroup) {
  const extras = [g.income > 0 && `Receitas ${formatBRL(g.income)}`, g.transfer > 0 && `Investimento ${formatBRL(g.transfer)}`].filter(
    (e): e is string => !!e,
  )
  if (g.expense > 0) return { amount: `-${formatBRL(g.expense)}`, extras }
  const [first, ...rest] = extras
  return { amount: first ?? formatBRL(0), extras: rest }
}

/** Lançamentos de um grupo aberto; busca só quando o card é expandido. */
function GroupDetails({ query, hideCategory = false }: { query: string; hideCategory?: boolean }) {
  const result = useTransactions(query)
  return (
    <QueryState query={result}>
      {(page) => (
        <>
          <TransactionTable items={page.items} hideCategory={hideCategory} />
          {page.total > page.items.length && (
            <p className="tile-note">Mostrando {page.items.length} de {page.total}. Refine os filtros para ver o restante.</p>
          )}
        </>
      )}
    </QueryState>
  )
}

function ListView({ query, onPage }: { query: string; onPage: (n: number) => void }) {
  const result = useTransactions(query)
  return (
    <QueryState query={result}>
      {(page) => (
        <section className={`card chart-body ${result.isPlaceholderData ? 'is-stale' : ''}`} aria-label="Lista de transações">
          {page.items.length === 0 ? <p className="state">Nenhuma transação encontrada.</p> : <TransactionTable items={page.items} />}
          <Pager page={page.page} limit={page.limit} total={page.total} onPage={onPage} />
        </section>
      )}
    </QueryState>
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
