import { describe, expect, it } from 'vitest'
import { canDecide, savingStatus } from './savings'
import type { SavingDecision } from '../api'

const base: SavingDecision = {
  kind: 'FIXED',
  key: 'streaming',
  label: 'STREAMING',
  category: 'ENTERTAINMENT',
  categoryLabel: 'Lazer',
  month: '2026-07',
  monthly: 40,
  status: 'CONFIRMED',
  monthsConfirmed: 3,
  realized: 120,
  returned: 0,
}

describe('canDecide', () => {
  it('só as sugestões recorrentes', () => {
    expect(['INCREASE', 'FIXED', 'ANT', 'DUPLICATE', 'NEW'].map((k) => canDecide(k as never))).toEqual([false, true, true, false, false])
  })
})

describe('savingStatus', () => {
  it('confirmada, voltou e aguardando', () => {
    const ok = savingStatus(base)
    expect(ok.tone).toBe('ok')
    expect(ok.text).toContain('3 meses sem a cobrança')
    expect(savingStatus({ ...base, monthsConfirmed: 1, realized: 40 }).text).toContain('1 mês sem a cobrança')

    const back = savingStatus({ ...base, status: 'RETURNED', returned: 44.9 })
    expect(back.tone).toBe('critical')
    expect(back.text).toContain('voltou')

    const wait = savingStatus({ ...base, status: 'PENDING', monthsConfirmed: 0, realized: 0 })
    expect(wait.tone).toBe('warning')
    expect(wait.text).toContain('julho de 2026')
  })
})
