import type { SettingField, SettingGroup } from '../api'

export const WEEKDAY_LABEL: Record<string, string> = {
  monday: 'Segunda-feira',
  tuesday: 'Terça-feira',
  wednesday: 'Quarta-feira',
  thursday: 'Quinta-feira',
  friday: 'Sexta-feira',
  saturday: 'Sábado',
  sunday: 'Domingo',
}

export const SOURCE_LABEL: Record<SettingField['source'], string> = {
  db: 'salvo aqui',
  env: 'definido no ambiente',
  default: 'padrão',
}

/** Valor mostrado: o rascunho se houver, senão o salvo. */
export const currentValue = (field: SettingField, drafts: Record<string, string>) => drafts[field.key] ?? field.value

/** Só o que o usuário mudou de fato (valor igual ao salvo não vai; segredo vazio também não). */
export function changedValues(group: SettingGroup, drafts: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = {}
  for (const f of group.fields) {
    const draft = drafts[f.key]
    if (draft === undefined) continue
    if (f.kind === 'secret' ? draft.trim() === '' : draft === f.value) continue
    out[f.key] = f.kind === 'secret' ? draft.trim() : draft
  }
  return out
}

/** Alguma das chaves exige confirmar a senha? */
export const needsPassword = (group: SettingGroup, keys: string[]) => group.fields.some((f) => f.sensitive && keys.includes(f.key))

/** O que o histórico mostra: "Token do bot alterado". */
export const auditText = (action: 'set' | 'reset', label: string) => `${label} ${action === 'set' ? 'alterado' : 'restaurado ao padrão do ambiente'}`

/** O teste de conexão de cada grupo, quando existe. */
export const TEST_TARGET: Record<string, 'pluggy' | 'gemini' | 'telegram' | undefined> = {
  pluggy: 'pluggy',
  ai: 'gemini',
  telegram: 'telegram',
}

/** Texto de apoio do estado de um campo, sem depender de cor. */
export function fieldStatus(f: SettingField): string {
  if (f.kind === 'secret') return f.isSet ? 'Configurado' : 'Não configurado'
  return ''
}
