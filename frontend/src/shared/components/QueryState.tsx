import type { ReactNode } from 'react'

interface Query<T> {
  data: T | undefined
  isError: boolean
  refetch: () => unknown
}

/** Erro com "tentar de novo" ou carregamento; com dados em mãos, mostra o conteúdo. */
export function QueryState<T>({ query, children }: { query: Query<T>; children: (data: T) => ReactNode }) {
  if (query.data !== undefined) return <>{children(query.data)}</>
  if (query.isError) {
    return (
      <div className="state error" role="alert">
        Não foi possível carregar os dados.{' '}
        <button type="button" className="btn" onClick={() => query.refetch()}>
          Tentar de novo
        </button>
      </div>
    )
  }
  return <p className="state">Carregando…</p>
}
