import { postJSON } from './request'

export interface SyncResult {
  inserted: number
  reconciled: number
  existing: number
  positions: number
  /** Algum item falhou; o que deu certo foi atualizado. */
  partial: boolean
  /** Quando o Pluggy atualizou os dados do banco (o mais antigo entre os bancos); null se não informado. */
  dataAsOf: string | null
}

/** Copia para o app o que o Meu Pluggy já tem (a mesma sincronização do /sync do chat). */
export const syncNow = () => postJSON<SyncResult>('/api/sync')
