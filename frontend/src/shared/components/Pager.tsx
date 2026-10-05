type Props = {
  page: number
  limit: number
  total: number
  onPage: (n: number) => void
  /** Singular e plural do que está sendo listado. */
  noun?: [string, string]
}

export function Pager({ page, limit, total, onPage, noun = ['transação', 'transações'] }: Props) {
  const pages = Math.max(1, Math.ceil(total / limit))
  return (
    <div className="pager">
      <span>
        {total} {total === 1 ? noun[0] : noun[1]}
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
