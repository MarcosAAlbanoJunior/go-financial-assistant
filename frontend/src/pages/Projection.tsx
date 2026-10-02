import { Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useSearchParams } from 'react-router'
import { useProjection } from '../api/client'
import { QueryState } from '../components/QueryState'
import { ScenarioForm } from '../components/ScenarioForm'
import { StatTile } from '../components/StatTile'
import { TrendChart } from '../components/TrendChart'
import { BUDGET, type ChartRow, type Series } from '../lib/chart'
import { formatBRL, formatMonthLong, formatMonthShort, formatPercent } from '../lib/format'
import { currentMonth } from '../lib/months'
import { costOf, defaultStart, loadPremises, loadScenarios, savePremises, saveScenarios, simulate, verdict, type Premises, type Row, type Scenario } from '../lib/simulation'

const RANGES = [6, 12, 24]

const COMPOSITION: Series[] = [...BUDGET, { key: 'scenario', name: 'Simulado', color: 'var(--series-4)' }]

const toRows = (rows: Row[]): ChartRow[] =>
  rows.map((r) => ({
    month: r.month, label: formatMonthShort(r.month),
    fixed: r.fixed, installment: r.installment, variable: r.variable, scenario: r.scenario,
    base: r.base, withScenario: r.withScenario,
  }))

export default function Projection() {
  const [params, setParams] = useSearchParams()
  const requested = Number(params.get('meses'))
  const months = RANGES.includes(requested) ? requested : 12
  const query = useProjection(months)

  const [scenarios, setScenarios] = useState<Scenario[]>(loadScenarios)
  const [adding, setAdding] = useState(false)
  const [override, setOverride] = useState<Partial<Premises>>(loadPremises)
  const updateOverride = (next: Partial<Premises>) => {
    setOverride(next)
    savePremises(next)
  }

  const update = (next: Scenario[]) => {
    setScenarios(next)
    saveScenarios(next)
  }

  return (
    <>
      <h1 className="page-title">Projeção e simulador</h1>
      <p className="tile-note">
        Parte do que você costuma gastar e das parcelas que já existem, e mostra como ficam os próximos meses com um novo financiamento. Os
        cenários ficam salvos só neste navegador.
      </p>

      <div className="filters" role="group" aria-label="Período">
        {RANGES.map((n) => (
          <button
            key={n}
            type="button"
            className="btn"
            aria-pressed={months === n}
            onClick={() => setParams((prev) => { const next = new URLSearchParams(prev); next.set('meses', String(n)); return next })}
          >
            {n} meses
          </button>
        ))}
      </div>

      <QueryState query={query}>
        {(projection) => {
          const a = projection.assumptions
          const premises: Premises = {
            income: override.income ?? a.income,
            fixed: override.fixed ?? a.fixed,
            variable: override.variable ?? a.variable,
          }
          const rows = simulate(projection, premises, scenarios)
          const v = verdict(rows)
          const active = scenarios.filter((s) => s.active)
          const chartRows = toRows(rows)
          const stale = query.isPlaceholderData

          return (
            <>
              <section className="card" aria-labelledby="prem-title">
                <h2 className="chart-title" id="prem-title">Premissas de um mês comum</h2>
                <p className="tile-note">
                  {a.basedOn > 0
                    ? `Baseadas nos últimos ${a.basedOn} ${a.basedOn === 1 ? 'mês' : 'meses'} com dados. Edite para testar outros cenários (ex.: renda menor); o que você editar fica salvo neste navegador.`
                    : 'Ainda não há histórico: informe os valores abaixo.'}
                </p>
                {a.incomeSources.length > 0 && (
                  <p className="tile-note">
                    A renda é a mediana da renda total dos últimos meses, então um mês fora do padrão (13º, adiantamento de férias) não pesa. Ela se compõe de:{' '}
                    {a.incomeSources.map((s) => `${s.label} ${formatBRL(s.monthly)}`).join(' · ')}.
                  </p>
                )}
                <div className="form-grid">
                  <PremiseField label="Renda mensal" value={premises.income} base={a.income} onChange={(x) => updateOverride({ ...override, income: x })} />
                  <PremiseField label="Contas fixas" value={premises.fixed} base={a.fixed} onChange={(x) => updateOverride({ ...override, fixed: x })} />
                  <PremiseField label="Gastos variáveis" value={premises.variable} base={a.variable} onChange={(x) => updateOverride({ ...override, variable: x })} />
                </div>
                {Object.keys(override).length > 0 && (
                  <button type="button" className="link" onClick={() => updateOverride({})}>
                    Voltar a todos os valores calculados
                  </button>
                )}
              </section>

              <div className="bill-section">
                <h2 className="bill-head">
                  Financiamentos simulados <span className="count">{scenarios.length}</span>
                  {!adding && (
                    <button type="button" className="btn btn-small" onClick={() => setAdding(true)}>
                      <Plus size={14} aria-hidden="true" /> Adicionar
                    </button>
                  )}
                </h2>
                {adding && (
                  <ScenarioForm
                    defaultStart={defaultStart(currentMonth())}
                    onCancel={() => setAdding(false)}
                    onSubmit={(s) => {
                      update([...scenarios, s])
                      setAdding(false)
                    }}
                  />
                )}
                {scenarios.length === 0 && !adding && <p className="state">Nenhum financiamento simulado. Adicione um para ver o efeito nos próximos meses.</p>}
                <div className="bill-grid">
                  {scenarios.map((s) => (
                    <ScenarioCard
                      key={s.id}
                      s={s}
                      onToggle={() => update(scenarios.map((x) => (x.id === s.id ? { ...x, active: !x.active } : x)))}
                      onRemove={() => update(scenarios.filter((x) => x.id !== s.id))}
                    />
                  ))}
                </div>
              </div>

              {v && (
                <div className="tiles section-gap">
                  <StatTile
                    label="Pior mês"
                    value={formatBRL(v.worst.balance)}
                    note={`${formatMonthLong(v.worst.month)} · ${v.worst.balance < 0 ? 'fica no vermelho' : 'ainda sobra'}`}
                  />
                  <StatTile label="Sobra média por mês" value={formatBRL(v.averageLeft)} note={active.length ? 'Com os financiamentos ativos' : 'Sem simulação'} />
                  <StatTile
                    label="Meses no vermelho"
                    value={String(v.negativeMonths)}
                    note={v.firstNegative ? `Começa em ${formatMonthLong(v.firstNegative)}${v.negativeMonthsBase ? ` (sem a simulação: ${v.negativeMonthsBase})` : ''}` : 'Nenhum nos meses projetados'}
                  />
                  {active.length > 0 && (
                    <StatTile
                      label="Peso na renda"
                      value={formatPercent(v.peakShareOfIncome)}
                      note={`Total pago: ${formatBRL(active.reduce((sum, s) => sum + costOf(s).total, 0))}`}
                    />
                  )}
                </div>
              )}

              <TrendChart title="Para onde vai o dinheiro, mês a mês" series={active.length ? COMPOSITION : BUDGET} variant="stack" rows={chartRows} stale={stale} />
              <TrendChart
                title="Saldo projetado (renda − despesas)"
                series={
                  active.length
                    ? [
                        { key: 'base', name: 'Sem os financiamentos', color: 'var(--series-1)' },
                        { key: 'withScenario', name: 'Com os financiamentos', color: 'var(--series-2)' },
                      ]
                    : [{ key: 'base', name: 'Saldo projetado', color: 'var(--series-1)' }]
                }
                variant="lines"
                rows={chartRows}
                stale={stale}
              />
              <p className="notice" role="note">
                <span aria-hidden="true">ⓘ</span>{' '}
                É uma estimativa: usa médias do passado e só as parcelas que já aparecem nos lançamentos (as do cartão são inferidas do
                &quot;n/m&quot; da descrição). Gastos novos, reajustes e rendimentos não entram.
              </p>
            </>
          )
        }}
      </QueryState>
    </>
  )
}

function PremiseField({ label, value, base, onChange }: { label: string; value: number; base: number; onChange: (v: number) => void }) {
  return (
    <label>
      {label} (R$)
      <input type="number" min={0} step="0.01" value={Math.round(value * 100) / 100} onChange={(e) => onChange(Math.max(0, Number(e.target.value) || 0))} />
      {Math.abs(value - base) > 0.005 && (
        <button type="button" className="link" onClick={() => onChange(base)}>
          voltar para {formatBRL(base)}
        </button>
      )}
    </label>
  )
}

function ScenarioCard({ s, onToggle, onRemove }: { s: Scenario; onToggle: () => void; onRemove: () => void }) {
  const cost = costOf(s)
  return (
    <article className="bill" style={{ '--c': 'var(--series-4)', opacity: s.active ? 1 : 0.6 } as React.CSSProperties}>
      <div className="bill-top">
        <span className="bill-name">
          <strong>{s.name}</strong>
          <span className="bill-meta">
            {s.parcels}× a partir de {formatMonthLong(s.start)}
          </span>
        </span>
        <span className="bill-amount">{formatBRL(cost.payment)}</span>
      </div>
      <div className="bill-foot">
        <span>Total {formatBRL(cost.total)}</span>
        {cost.interest > 0 && <span>juros {formatBRL(cost.interest)}</span>}
        {cost.down > 0 && <span>entrada {formatBRL(cost.down)}</span>}
      </div>
      <div className="bill-foot">
        <label className="check">
          <input type="checkbox" checked={s.active} onChange={onToggle} /> Considerar na projeção
        </label>
        <button type="button" className="btn btn-small" onClick={onRemove} aria-label={`Remover ${s.name}`}>
          <Trash2 size={13} aria-hidden="true" /> Remover
        </button>
      </div>
    </article>
  )
}
