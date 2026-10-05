import { describe, expect, it } from 'vitest'
import { auditText, changedValues, currentValue, fieldStatus, needsConfirmation } from './settings'
import type { SettingField, SettingGroup } from '../api'

const f = (key: string, kind: SettingField['kind'], value = '', isSet = value !== ''): SettingField => ({
  key, label: key, help: '', kind, live: true, value, isSet, source: 'default', pendingRestart: false, default: '', sensitive: kind === 'secret',
})
const group: SettingGroup = {
  id: 'g', title: 'G', help: '',
  fields: [f('DIGEST_HOUR', 'int', '9'), f('OWN_NAMES', 'list', 'Maria Silva'), f('PLUGGY_CLIENT_SECRET', 'secret', '', true), f('GEMINI_PAID_PLAN', 'bool', 'false')],
}

describe('settings', () => {
  it('envia só o que mudou', () => {
    expect(changedValues(group, {})).toEqual({})
    expect(changedValues(group, { DIGEST_HOUR: '9', OWN_NAMES: 'Maria Silva' })).toEqual({})
    expect(changedValues(group, { DIGEST_HOUR: '10', GEMINI_PAID_PLAN: 'true' })).toEqual({ DIGEST_HOUR: '10', GEMINI_PAID_PLAN: 'true' })
  })

  it('campo de texto esvaziado conta como mudança (limpar os nomes)', () => {
    expect(changedValues(group, { OWN_NAMES: '' })).toEqual({ OWN_NAMES: '' })
  })

  it('segredo vazio não muda nada; preenchido vai sem espaços nas pontas', () => {
    expect(changedValues(group, { PLUGGY_CLIENT_SECRET: '   ' })).toEqual({})
    expect(changedValues(group, { PLUGGY_CLIENT_SECRET: ' novo ' })).toEqual({ PLUGGY_CLIENT_SECRET: 'novo' })
  })

  it('o rascunho vale sobre o valor salvo', () => {
    expect(currentValue(group.fields[0], {})).toBe('9')
    expect(currentValue(group.fields[0], { DIGEST_HOUR: '' })).toBe('')
  })

  it('segredo mostra só se está configurado', () => {
    expect(fieldStatus(group.fields[2])).toBe('Configurado')
    expect(fieldStatus(f('X_SECRET', 'secret'))).toBe('Não configurado')
    expect(fieldStatus(group.fields[0])).toBe('')
  })

  it('só pede a senha quando uma chave sensível mudou', () => {
    expect(needsConfirmation(group, ['DIGEST_HOUR', 'OWN_NAMES'])).toBe(false)
    expect(needsConfirmation(group, ['DIGEST_HOUR', 'PLUGGY_CLIENT_SECRET'])).toBe(true)
    expect(needsConfirmation(group, [])).toBe(false)
  })

  it('descreve o histórico sem valores', () => {
    expect(auditText('set', 'Token do bot')).toBe('Token do bot alterado')
    expect(auditText('reset', 'Token do bot')).toContain('restaurado')
  })
})
