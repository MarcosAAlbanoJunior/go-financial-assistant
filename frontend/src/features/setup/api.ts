import { useQuery } from '@tanstack/react-query'
import { postJSON, request } from '../../shared/api/request'

export type ProblemKind = 'no-token' | 'short-token' | 'no-key' | 'database'

export interface TelegramState {
  bot: string
  candidate: { name: string; username: string } | null
  accepted: boolean
}

/** Estado do setup. Sem sessão de setup (token ainda não colado), só vêm os campos do topo. */
export interface SetupStatus {
  open: boolean
  reopen: boolean
  session: boolean
  tokenMinLength: number
  passwordMinLength: number
  problem?: string
  problemKind?: ProblemKind
  password?: { draft: boolean; fromEnv: boolean; current: boolean }
  channel?: { configured: boolean; canReplace: boolean }
  telegram?: TelegramState
}

// Sem retry: 404 é "setup concluído" (vai para o login), e repetir não muda o resultado.
export const useSetupStatus = (enabled = true) =>
  useQuery({ queryKey: ['setup-status'], queryFn: () => request<SetupStatus>('/api/setup/status'), retry: false, enabled })

export const sendToken = (token: string) => postJSON<{ ok: boolean }>('/api/setup/token', { token })
export const savePassword = (password: string) => postJSON<{ ok: boolean }>('/api/setup/password', { password })
export const saveBot = (token: string) => postJSON<{ bot: string }>('/api/setup/telegram/bot', { token })
export const pollStart = () => postJSON<TelegramState>('/api/setup/telegram/poll')
export const rejectCandidate = () => postJSON<{ ok: boolean }>('/api/setup/telegram/reject')
export const acceptCandidate = () => postJSON<{ step: 'code' }>('/api/setup/telegram/accept')
export const confirmCode = (code: string) => postJSON<{ ok: boolean }>('/api/setup/telegram/confirm', { code })
export const keepChannel = () => postJSON<{ ok: boolean }>('/api/setup/keep')
