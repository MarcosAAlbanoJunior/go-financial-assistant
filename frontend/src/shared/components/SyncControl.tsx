import { RefreshCw } from 'lucide-react'
import { ApiError } from '../api/request'
import type { SyncResult } from '../api/sync'
import { freshnessText } from '../lib/format'
import { useSync } from '../lib/useSync'

const MEU_PLUGGY = 'https://meu.pluggy.ai'

function errorText(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 409) return 'Já existe uma sincronização em andamento. Tente em instantes.'
    if (error.status === 503) return 'O Open Finance não está configurado (veja Configurações).'
    if (error.status === 429) return 'Muitas tentativas seguidas. Espere um minuto.'
  }
  return 'Não foi possível sincronizar agora.'
}

function resultText(r: SyncResult): string {
  const news = r.inserted === 0 ? 'Nada novo' : `${r.inserted} ${r.inserted === 1 ? 'lançamento novo' : 'lançamentos novos'}`
  const reconciled = r.reconciled > 0 ? `, ${r.reconciled} conciliado(s)` : ''
  return `${news}${reconciled}.`
}

/**
 * Botão global de sincronização (menu lateral e barra do celular). Sincronizar copia o que o Meu Pluggy já tem: ele busca no
 * banco sozinho (cerca de 1x/dia) ou quando a pessoa pede no app dele, e a API não permite forçar. Por isso a resposta
 * mostra a idade real dos dados do banco.
 */
export function SyncControl({ compact = false }: { compact?: boolean }) {
  const sync = useSync()

  if (compact) {
    return (
      <button type="button" className="icon-btn" aria-label="Sincronizar" disabled={sync.pending} onClick={() => sync.mutate()}>
        <RefreshCw size={20} className={sync.pending ? 'sync-spin' : undefined} aria-hidden="true" />
      </button>
    )
  }

  return (
    <div className="sync-control">
      <button type="button" className="btn sync-btn" disabled={sync.pending} onClick={() => sync.mutate()}>
        <RefreshCw size={16} className={sync.pending ? 'sync-spin' : undefined} aria-hidden="true" /> {sync.pending ? 'Sincronizando…' : 'Sincronizar'}
      </button>
      <div role="status" className="sync-status">
        {sync.isSuccess && !sync.pending && (
          <>
            <p>{resultText(sync.data)}</p>
            {sync.data.dataAsOf && <p>Dados do banco de {freshnessText(sync.data.dataAsOf, new Date(sync.submittedAt))}.</p>}
            {sync.data.partial && <p className="sync-warn">Alguns bancos falharam; os demais foram atualizados.</p>}
            <p>
              Não achou o que mudou agora?{' '}
              <a href={MEU_PLUGGY} target="_blank" rel="noopener noreferrer">
                Atualize no Meu Pluggy
              </a>{' '}
              e sincronize de novo.
            </p>
          </>
        )}
        {sync.isError && <p className="error">{errorText(sync.error)}</p>}
      </div>
    </div>
  )
}
