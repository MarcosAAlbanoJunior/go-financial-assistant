import { Check, Clock, Undo2 } from 'lucide-react'
import type { CSSProperties } from 'react'
import type { BudgetItem } from '../api/types'
import { CLASS_META, cleanLabel } from '../lib/budget'
import { categoryVisual } from '../lib/categoryVisual'
import { formatBRL } from '../lib/format'
import { CategoryChip } from './CategoryChip'

interface Props {
  item: BudgetItem
  busy: boolean
  /** Corrige a classe à mão; "AUTO" volta à detecção automática. */
  onRule: (key: string, cls: 'FIXED' | 'VARIABLE' | 'AUTO') => void
}

/** Uma conta do mês: ícone da categoria, valor, dia, situação e a correção manual da classe. */
export function BillCard({ item, busy, onRule }: Props) {
  const meta = CLASS_META[item.class]
  return (
    <article className="bill" style={{ '--c': meta.color, '--cat': categoryVisual(item.category).color } as CSSProperties}>
      <div className="bill-top">
        <CategoryChip category={item.category} size={20} />
        <span className="bill-name">
          <strong>{cleanLabel(item.label)}</strong>
          <span className="bill-meta">
            {item.categoryLabel}
            {item.count > 1 ? ` · ${item.count} lançamentos` : ''}
          </span>
        </span>
        <span className="bill-amount">{formatBRL(item.total)}</span>
      </div>
      <div className="bill-foot">
        {item.day > 0 && <span>dia {item.day}</span>}
        <span className={`status ${item.paid ? 'status-paid' : 'status-pending'}`}>
          {item.paid ? <Check size={12} aria-hidden="true" /> : <Clock size={12} aria-hidden="true" />}
          {item.paid ? 'Pago' : 'Pendente'}
        </span>
        {item.months > 1 && <span>em {item.months} meses</span>}
        {item.manual && <span className="badge">marcada por você</span>}
        {item.manual ? (
          <button type="button" className="btn btn-small" disabled={busy} onClick={() => onRule(item.key, 'AUTO')}>
            <Undo2 size={13} aria-hidden="true" /> Voltar ao automático
          </button>
        ) : item.class === 'FIXED' ? (
          <button type="button" className="btn btn-small" disabled={busy} onClick={() => onRule(item.key, 'VARIABLE')}>
            Não é fixa
          </button>
        ) : item.class === 'VARIABLE' ? (
          <button type="button" className="btn btn-small" disabled={busy} onClick={() => onRule(item.key, 'FIXED')}>
            Marcar como fixa
          </button>
        ) : null}
      </div>
    </article>
  )
}
