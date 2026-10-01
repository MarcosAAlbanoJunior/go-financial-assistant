import type { Transaction } from '../api/types'
import { describeAmount, formatDay } from '../lib/transactions'
import { CategoryChip } from './CategoryChip'

/** Tabela de lançamentos. Em telas estreitas categoria e conta descem para baixo da descrição. */
export function TransactionTable({ items, hideCategory = false }: { items: Transaction[]; hideCategory?: boolean }) {
  return (
    <div className="table-scroll">
      <table className="data tx">
        <thead>
          <tr>
            <th scope="col">Data</th>
            <th scope="col">Descrição</th>
            {!hideCategory && (
              <th scope="col" className="col-wide">
                Categoria
              </th>
            )}
            <th scope="col" className="col-wide">
              Pagamento
            </th>
            <th scope="col" className="col-wide">
              Conta
            </th>
            <th scope="col">Valor</th>
          </tr>
        </thead>
        <tbody>
          {items.map((t) => {
            const a = describeAmount(t)
            return (
              <tr key={t.id}>
                <td>{formatDay(t.date)}</td>
                <td className="left">
                  {t.description || '—'}
                  {t.status === 'PENDING' && <span className="badge">Pendente</span>}
                  {t.source === 'MANUAL' && <span className="badge">Manual</span>}
                  <span className="tx-meta">{[hideCategory ? '' : t.categoryLabel, t.accountName].filter(Boolean).join(' · ')}</span>
                </td>
                {!hideCategory && (
                  <td className="left col-wide">
                    <span className="cat-cell">
                      <CategoryChip category={t.category} size={14} />
                      {t.categoryLabel}
                    </span>
                  </td>
                )}
                <td className="left col-wide">{t.paymentMethodLabel}</td>
                <td className="left col-wide">{t.accountName || '—'}</td>
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
  )
}
