import { describe, expect, it } from 'vitest'
import { ApiError } from '../../../shared/api/request'
import { currentStep, passwordStrength, setupErrorMessage, stepNumber, telegramStage } from './setup'
import type { SetupStatus } from '../api'

const base: SetupStatus = { open: true, reopen: false, session: true, tokenMinLength: 24, passwordMinLength: 12 }
const pw = (p: Partial<NonNullable<SetupStatus['password']>>) => ({ draft: false, fromEnv: false, current: false, ...p })

describe('passo atual', () => {
  it('sem sessão de setup, começa (ou recomeça) pelo token', () => {
    expect(currentStep({ ...base, session: false, password: pw({ draft: true }) })).toBe('token')
  })
  it('sem senha resolvida, pede a senha', () => {
    expect(currentStep({ ...base, password: pw({}) })).toBe('password')
  })
  it('senha nova, do .env ou a de antes (reaberto) seguem para o canal', () => {
    for (const p of [pw({ draft: true }), pw({ fromEnv: true }), pw({ current: true })]) {
      expect(currentStep({ ...base, password: p })).toBe('channel')
    }
  })
  it('"trocar" a senha volta ao passo dela', () => {
    expect(currentStep({ ...base, password: pw({ draft: true }) }, true)).toBe('password')
  })
  it('numera os passos de 1 a 4', () => {
    expect(stepNumber('token')).toBe(1)
    expect(stepNumber('done')).toBe(4)
  })
})

describe('etapa do Telegram', () => {
  it('segue bot → /start → é você? → código', () => {
    expect(telegramStage(undefined)).toBe('bot')
    expect(telegramStage({ bot: '', candidate: null, accepted: false })).toBe('bot')
    expect(telegramStage({ bot: 'meu_bot', candidate: null, accepted: false })).toBe('start')
    expect(telegramStage({ bot: 'meu_bot', candidate: { name: 'Ana', username: '' }, accepted: false })).toBe('confirm-person')
    expect(telegramStage({ bot: 'meu_bot', candidate: { name: 'Ana', username: '' }, accepted: true })).toBe('code')
  })
})

describe('medidor de senha', () => {
  it('abaixo do mínimo diz quanto falta', () => {
    expect(passwordStrength('abc', 12)).toEqual({ score: 0, label: 'Curta: faltam 9 caracteres' })
    expect(passwordStrength('abcdefghijk', 12).label).toBe('Curta: faltam 1 caractere')
  })
  it('melhora com tamanho e variedade', () => {
    expect(passwordStrength('abcdefghijkl', 12).score).toBe(1)
    expect(passwordStrength('abcdefghij1!', 12).score).toBe(2)
    expect(passwordStrength('uma frase bem longa aqui', 12).score).toBe(3)
  })
  it('conta caracteres, não bytes', () => {
    expect(passwordStrength('ção'.repeat(4), 12).score).toBeGreaterThan(0)
  })
})

describe('mensagens de erro', () => {
  it('as do servidor ganham maiúscula e ponto', () => {
    expect(setupErrorMessage(new ApiError(401, 'token de setup incorreto'))).toBe('Token de setup incorreto.')
    expect(setupErrorMessage(new ApiError(400, 'token inválido: confira se copiou inteiro (formato 123456789:ABC...)'))).toMatch(/\.\.\.\)\.$/)
  })
  it('o limite por IP (sem JSON) vira uma mensagem clara', () => {
    expect(setupErrorMessage(new ApiError(429, 'erro inesperado'))).toMatch(/Aguarde um minuto/)
  })
  it('sem resposta do app', () => {
    expect(setupErrorMessage(new TypeError('Failed to fetch'))).toMatch(/no ar/)
    expect(setupErrorMessage(null)).toBeNull()
  })
})
