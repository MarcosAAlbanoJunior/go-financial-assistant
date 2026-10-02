import { useIsMutating, useMutation, useQueryClient } from '@tanstack/react-query'
import { syncNow } from '../api/sync'

const KEY = ['sync']

/** Sincronização global: todos os botões compartilham o estado "sincronizando"; ao terminar, recarrega os dados das telas. */
export function useSync() {
  const queryClient = useQueryClient()
  const pending = useIsMutating({ mutationKey: KEY }) > 0
  const mutation = useMutation({ mutationKey: KEY, mutationFn: syncNow, onSuccess: () => queryClient.invalidateQueries() })
  return { ...mutation, pending }
}
