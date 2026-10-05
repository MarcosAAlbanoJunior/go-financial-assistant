import { useQuery } from '@tanstack/react-query'
import { postJSON, request } from '../../shared/api/request'

export type SettingKind = 'text' | 'int' | 'bool' | 'enum' | 'list' | 'secret'

export interface SettingField {
  key: string
  label: string
  help: string
  kind: SettingKind
  options?: string[]
  min?: number
  max?: number
  /** Vale na hora; senão, só depois de reiniciar o app. */
  live: boolean
  /** Vazio para segredos: eles nunca voltam para a tela. */
  value: string
  isSet: boolean
  source: 'db' | 'env' | 'default'
  pendingRestart: boolean
  default: string
  /** Mudar ou restaurar exige confirmar a senha do dashboard. */
  sensitive: boolean
}

export interface SettingGroup {
  id: string
  title: string
  help: string
  fields: SettingField[]
}

export interface SettingsData {
  channel: string
  /** Sem APP_SECRET_KEY no ambiente, segredos não podem ser guardados aqui. */
  encryption: boolean
  restartPending: boolean
  groups: SettingGroup[]
  /** Com o segundo fator, o que é sensível se confirma com um código no chat; sem ele, com a senha. */
  secondFactor: boolean
}

/** A prova pedida para mudar o que é sensível. */
export type Proof = { password: string } | { code: string }

export interface TransferExample {
  description: string
  kind: 'EXPENSE' | 'INCOME'
  amount: number
}

export interface OwnTransfers {
  count: number
  expense: number
  income: number
  examples: TransferExample[]
}

export interface SaveSettingsResult {
  changed: string[]
  restartPending: boolean
  ownTransfers?: OwnTransfers
}

export interface SettingsAuditEntry {
  at: string
  action: 'set' | 'reset' | 'setup'
  key: string
  label: string
  sensitive: boolean
  ip: string
}

export const useSettings = () => useQuery({ queryKey: ['settings'], queryFn: () => request<SettingsData>('/api/settings') })

/** Salva só o que mudou. Ao mudar OWN_NAMES, a resposta traz as transferências antigas que parecem ser suas. */
export const saveSettings = (values: Record<string, string>, proof?: Proof) =>
  request<SaveSettingsResult>('/api/settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ values, ...proof }),
  })

/** Apaga o valor salvo aqui: volta a valer o ambiente (ou o padrão). */
export const resetSetting = (key: string, proof?: Proof) => postJSON(`/api/settings/reset/${encodeURIComponent(key)}`, { ...proof })

/** Manda ao chat o código que confirma a próxima alteração sensível. */
export const requestConfirmCode = () => postJSON<{ step: 'code' }>('/api/settings/confirm')

/** Últimas alterações de configuração (sem valores). */
export const useSettingsAudit = () => useQuery({ queryKey: ['settings-audit'], queryFn: () => request<SettingsAuditEntry[]>('/api/settings/audit') })

export const applyOwnTransfers = () => postJSON<{ cancelled: number }>('/api/settings/own-transfers/apply')

export const testConnection = (target: 'pluggy' | 'gemini' | 'telegram') =>
  postJSON<{ ok: boolean; message: string }>(`/api/settings/test/${target}`)

export const restartApp = () => postJSON('/api/restart')
