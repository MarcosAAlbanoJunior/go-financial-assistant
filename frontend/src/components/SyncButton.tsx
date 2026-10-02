import { useMutation, useQueryClient } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { ApiError, syncNow } from '../api/client'

function message(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 409) return 'Já existe uma sincronização em andamento. Tente em instantes.'
    if (error.status === 503) return 'O Open Finance não está configurado (veja o README).'
    if (error.status === 429) return 'Muitas tentativas seguidas. Espere um minuto.'
  }
  return 'Não foi possível sincronizar agora.'
}

/** Pede a mesma sincronização do /sync e, ao terminar, recarrega os dados da tela. `stale` destaca o botão. */
export function SyncButton({ highlight }: { highlight: boolean }) {
  const queryClient = useQueryClient()
  const sync = useMutation({
    mutationFn: syncNow,
    onSuccess: () => queryClient.invalidateQueries(),
  })

  return (
    <>
      <button type="button" className={`btn bal-btn ${highlight ? 'btn-primary' : ''}`} disabled={sync.isPending} onClick={() => sync.mutate()}>
        <RefreshCw size={16} className={sync.isPending ? 'bal-spin' : undefined} aria-hidden="true" /> {sync.isPending ? 'Sincronizando…' : 'Sincronizar'}
      </button>
      <span className="sr-only" role="status">
        {sync.isPending ? 'Sincronizando com o Open Finance' : sync.isSuccess ? 'Sincronização concluída' : ''}
      </span>
      {sync.isError && (
        <p className="error bal-sync-msg" role="alert">
          {message(sync.error)}
        </p>
      )}
      {sync.isSuccess && sync.data.partial && (
        <p className="notice bal-sync-msg" role="status">
          Alguns bancos falharam ao sincronizar; os demais foram atualizados.
        </p>
      )}
    </>
  )
}
