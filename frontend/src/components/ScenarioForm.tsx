import { useState, type FormEvent } from 'react'
import { costOf, type Scenario } from '../lib/simulation'
import { formatBRL } from '../lib/format'

interface Props {
  defaultStart: string
  onSubmit: (s: Scenario) => void
  onCancel: () => void
}

const num = (v: string) => (v === '' ? 0 : Number(v))

/** Formulário de um financiamento: ou a parcela já conhecida, ou valor, entrada, taxa e prazo para calcular. */
export function ScenarioForm({ defaultStart, onSubmit, onCancel }: Props) {
  const [name, setName] = useState('')
  const [mode, setMode] = useState<Scenario['mode']>('installment')
  const [start, setStart] = useState(defaultStart)
  const [parcels, setParcels] = useState('12')
  const [payment, setPayment] = useState('')
  const [price, setPrice] = useState('')
  const [down, setDown] = useState('')
  const [rate, setRate] = useState('1.5')

  const draft: Scenario = {
    id: '',
    name: name.trim() || 'Financiamento',
    mode,
    start,
    parcels: Math.trunc(num(parcels)),
    payment: num(payment),
    price: num(price),
    down: num(down),
    ratePct: num(rate),
    active: true,
  }
  const cost = costOf(draft)
  const valid =
    /^\d{4}-(0[1-9]|1[0-2])$/.test(start) &&
    draft.parcels >= 1 && draft.parcels <= 480 &&
    (mode === 'installment' ? draft.payment > 0 : draft.price > 0 && draft.down < draft.price && draft.ratePct >= 0 && draft.ratePct <= 100) &&
    draft.name.length <= 60

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (valid) onSubmit({ ...draft, id: crypto.randomUUID() })
  }

  return (
    <form className="card scenario-form" onSubmit={handleSubmit} aria-label="Novo financiamento">
      <h2 className="chart-title">Simular um financiamento</h2>

      <div className="view-toggle" role="group" aria-label="Como informar">
        <button type="button" className="btn" aria-pressed={mode === 'installment'} onClick={() => setMode('installment')}>
          Já sei a parcela
        </button>
        <button type="button" className="btn" aria-pressed={mode === 'financing'} onClick={() => setMode('financing')}>
          Calcular pela taxa
        </button>
      </div>

      <div className="form-grid">
        <label>
          Nome
          <input type="text" maxLength={60} placeholder="Ex.: Carro" value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        <label>
          Primeira parcela em
          <input type="month" required value={start} onChange={(e) => setStart(e.target.value)} />
        </label>
        <label>
          Número de parcelas
          <input type="number" min={1} max={480} step={1} required value={parcels} onChange={(e) => setParcels(e.target.value)} />
        </label>
        {mode === 'installment' ? (
          <label>
            Valor da parcela (R$)
            <input type="number" min={0} step="0.01" required value={payment} onChange={(e) => setPayment(e.target.value)} />
          </label>
        ) : (
          <>
            <label>
              Valor do bem (R$)
              <input type="number" min={0} step="0.01" required value={price} onChange={(e) => setPrice(e.target.value)} />
            </label>
            <label>
              Entrada (R$)
              <input type="number" min={0} step="0.01" value={down} onChange={(e) => setDown(e.target.value)} />
            </label>
            <label>
              Juros ao mês (%)
              <input type="number" min={0} max={100} step="0.01" value={rate} onChange={(e) => setRate(e.target.value)} />
            </label>
          </>
        )}
      </div>

      {valid && (
        <p className="scenario-preview" aria-live="polite">
          Parcela de <strong>{formatBRL(cost.payment)}</strong> por {draft.parcels} {draft.parcels === 1 ? 'mês' : 'meses'}
          {cost.down > 0 && <> + entrada de <strong>{formatBRL(cost.down)}</strong></>}. Total pago: <strong>{formatBRL(cost.total)}</strong>
          {mode === 'financing' && cost.interest > 0 && <> (juros de {formatBRL(cost.interest)})</>}.
        </p>
      )}

      <div className="form-actions">
        <button type="submit" className="btn btn-primary" disabled={!valid}>
          Adicionar à simulação
        </button>
        <button type="button" className="btn" onClick={onCancel}>
          Cancelar
        </button>
      </div>
    </form>
  )
}
