import './styles.css'
import { BalanceHero } from './components/BalanceHero'
import { BalanceTable } from './components/BalanceTable'
import { HideValuesToggle } from './components/HideValuesToggle'
import { InstitutionCard } from './components/InstitutionCard'
import { useHideValues } from './lib/useHideValues'
import { useBalances } from './api'

export default function Panel() {
  const query = useBalances()
  const { hidden, toggle } = useHideValues()
  const data = query.data
  // "hoje" e "ontem" são relativos ao momento em que os dados chegaram.
  const now = new Date(query.dataUpdatedAt)

  return (
    <>
      <div className="bal-top">
        <h1 className="page-title">Painel</h1>
        <HideValuesToggle hidden={hidden} onToggle={toggle} />
      </div>

      {data ? (
        data.institutions.length === 0 ? (
          <p className="state">
            Nenhuma conta sincronizada. Configure o Open Finance (veja a seção Open Finance do README) e use “Sincronizar”.
          </p>
        ) : (
          <div className={`chart-body ${query.isFetching ? 'is-stale' : ''}`}>
            <BalanceHero data={data} hidden={hidden} now={now} />
            <div className="bal-grid">
              {data.institutions.map((i) => (
                <InstitutionCard key={i.id} institution={i} hidden={hidden} now={now} />
              ))}
            </div>
            <BalanceTable data={data} hidden={hidden} />
          </div>
        )
      ) : query.isError ? (
        <div className="state error" role="alert">
          Não foi possível carregar os saldos.{' '}
          <button type="button" className="btn" onClick={() => query.refetch()}>
            Tentar de novo
          </button>
        </div>
      ) : (
        <Skeleton />
      )}
    </>
  )
}

function Skeleton() {
  return (
    <div aria-busy="true" aria-label="Carregando saldos">
      <div className="bal-skel bal-skel-hero" />
      <div className="bal-grid">
        <div className="bal-skel bal-skel-card" />
        <div className="bal-skel bal-skel-card" />
      </div>
    </div>
  )
}
