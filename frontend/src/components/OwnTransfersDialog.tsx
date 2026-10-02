import { useEffect, useRef } from 'react'
import type { OwnTransfers } from '../api/types'
import { formatBRL } from '../lib/format'

interface Props {
  data: OwnTransfers
  busy: boolean
  onApply: () => void
  onKeep: () => void
}

/** Pergunta se os Pix/TED antigos com o seu nome devem deixar de contar. Nada é cancelado sem o "sim". */
export function OwnTransfersDialog({ data, busy, onApply, onKeep }: Props) {
  const ref = useRef<HTMLDialogElement>(null)
  useEffect(() => {
    const d = ref.current
    if (d && !d.open) d.showModal()
  }, [])

  const totals = [data.expense > 0 && `saídas: ${formatBRL(data.expense)}`, data.income > 0 && `entradas: ${formatBRL(data.income)}`].filter(Boolean).join('; ')

  return (
    <dialog ref={ref} className="set-dialog" aria-labelledby="own-title" onCancel={(e) => (busy ? e.preventDefault() : onKeep())}>
      <h2 id="own-title" className="chart-title">
        Pix entre contas suas encontrados
      </h2>
      <p>
        Já existem <strong>{data.count}</strong> lançamento(s) com o seu nome que parecem transferências entre contas suas ({totals}). Eles estão contando como gasto ou renda. Quer desconsiderá-los?
      </p>
      <ul className="set-examples">
        {data.examples.map((e, i) => (
          <li key={i}>
            {e.description} · {e.kind === 'EXPENSE' ? 'saída' : 'entrada'} {formatBRL(e.amount)}
          </li>
        ))}
        {data.count > data.examples.length && <li>…e mais {data.count - data.examples.length}</li>}
      </ul>
      <p className="tile-note">Eles ficam cancelados no banco (não são apagados) e deixam de entrar em gastos, renda, orçamento e projeção.</p>
      <div className="form-actions">
        <button type="button" className="btn btn-primary" disabled={busy} onClick={onApply}>
          {busy ? 'Aplicando…' : `Desconsiderar ${data.count} lançamento(s)`}
        </button>
        <button type="button" className="btn" disabled={busy} onClick={onKeep}>
          Manter como estão
        </button>
      </div>
    </dialog>
  )
}
