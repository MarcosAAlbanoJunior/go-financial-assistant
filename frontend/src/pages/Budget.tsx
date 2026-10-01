import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { setExpenseRule, useBudget, useSummary } from '../api/client'
import type { ExpenseClass } from '../api/types'
import { BillCard } from '../components/BillCard'
import { MonthFilter } from '../components/MonthFilter'
import { QueryState } from '../components/QueryState'
import { SplitBar } from '../components/SplitBar'
import { StatTile } from '../components/StatTile'
import { TrendChart } from '../components/TrendChart'
import { BUDGET, toBudgetRows } from '../lib/chart'
import { CLASS_META, CLASS_ORDER, committedShare, itemsOf, splitOf } from '../lib/budget'
import { formatBRL, formatMonthTitle, formatPercent } from '../lib/format'
import { useMonth } from '../lib/useMonth'

const VARIABLE_PREVIEW = 12

export default function Budget() {
  const { month, setMonth, now } = useMonth()
  const budget = useBudget(month)
  const summary = useSummary(month)
  const queryClient = useQueryClient()
  const [showAllVariable, setShowAllVariable] = useState(false)

  const rule = useMutation({
    mutationFn: ({ key, cls }: { key: string; cls: 'FIXED' | 'VARIABLE' | 'AUTO' }) => setExpenseRule(key, cls),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['budget'] }),
  })

  return (
    <>
      <h1 className="page-title">Orçamento de {formatMonthTitle(month)}</h1>
      <MonthFilter month={month} now={now} onChange={setMonth} />

      <QueryState query={budget}>
        {(b) => {
          const split = splitOf(b.series.at(-1))
          const income = summary.data?.current.income ?? 0
          const committed = committedShare(split, income)
          const stale = budget.isPlaceholderData
          return (
            <>
              <section className="card" aria-labelledby="split-title">
                <h2 className="chart-title" id="split-title">
                  Para onde foi o dinheiro
                </h2>
                <SplitBar
                  stale={stale}
                  parts={CLASS_ORDER.map((c) => ({
                    label: CLASS_META[c].label,
                    color: CLASS_META[c].color,
                    value: c === 'FIXED' ? split.fixed : c === 'INSTALLMENT' ? split.installment : split.variable,
                  }))}
                />
              </section>

              <div className="tiles section-gap">
                <StatTile label="Despesas do mês" value={formatBRL(split.total)} />
                <StatTile
                  label="Comprometido (fixas + parcelas)"
                  value={formatBRL(split.fixed + split.installment)}
                  note={committed === null ? 'Sem receita no mês para comparar' : `${formatPercent(committed)} da receita do mês`}
                />
                <StatTile label="Variáveis" value={formatBRL(split.variable)} note="O que dá para ajustar de um mês para o outro" />
              </div>

              <TrendChart title="Fixas, parceladas e variáveis por mês" series={BUDGET} variant="stack" rows={toBudgetRows(b.series)} stale={stale} />

              {rule.isError && (
                <p className="state error" role="alert">
                  Não foi possível salvar a correção. Tente de novo.
                </p>
              )}

              {CLASS_ORDER.map((cls) => {
                const all = itemsOf(b.items, cls)
                if (all.length === 0) return null
                const preview = cls === 'VARIABLE' && !showAllVariable ? all.slice(0, VARIABLE_PREVIEW) : all
                return (
                  <BillSection key={cls} cls={cls} total={all.length}>
                    {preview.map((item) => (
                      <BillCard key={item.key} item={item} busy={rule.isPending} onRule={(key, c) => rule.mutate({ key, cls: c })} />
                    ))}
                    {cls === 'VARIABLE' && all.length > VARIABLE_PREVIEW && (
                      <button type="button" className="btn" onClick={() => setShowAllVariable(!showAllVariable)}>
                        {showAllVariable ? 'Mostrar só as maiores' : `Mostrar todas (${all.length})`}
                      </button>
                    )}
                  </BillSection>
                )
              })}
              <p className="tile-note section-gap">
                Uma conta é considerada fixa quando se repete nos meses (até 2 vezes por mês, com valor parecido). Parceladas são as que têm
                &quot;n/m&quot; na descrição. Você pode corrigir uma conta à mão.
              </p>
            </>
          )
        }}
      </QueryState>
    </>
  )
}

function BillSection({ cls, total, children }: { cls: ExpenseClass; total: number; children: React.ReactNode }) {
  const { label, color, Icon } = CLASS_META[cls]
  return (
    <section className="bill-section" aria-label={label}>
      <h2 className="bill-head" style={{ color: 'var(--text-primary)' }}>
        <Icon size={18} style={{ color }} aria-hidden="true" /> {label} <span className="count">{total}</span>
      </h2>
      <div className="bill-grid">{children}</div>
    </section>
  )
}
