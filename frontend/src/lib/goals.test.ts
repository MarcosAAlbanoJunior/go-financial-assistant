import { describe, expect, it } from 'vitest'
import type { Goal } from '../api/types'
import { emptyGoalForm, goalStatus, progressRatio, toGoalInput } from './goals'

const base: Goal = {
  id: 'x', kind: 'SAVE', name: 'Viagem', targetDate: '2027-01', category: '', categoryLabel: '', cutPercent: 0, baseline: 0,
  reserveMonths: 0, current: 4000, target: 10000, done: false, monthsLeft: 3, perMonth: 2000, surplus: 1300, fits: false, coverage: 0, history: [],
}
const g = (over: Partial<Goal>): Goal => ({ ...base, ...over })
const form = emptyGoalForm('2026-11')

describe('toGoalInput', () => {
  it('monta o corpo de cada tipo', () => {
    expect(toGoalInput({ ...form, name: ' Viagem ', amount: '5000', date: '2027-06' }, '2026-10')).toEqual({
      input: { kind: 'SAVE', name: 'Viagem', targetAmount: 5000, targetDate: '2027-06' },
    })
    expect(toGoalInput({ ...form, kind: 'CUT', name: 'Comida', percent: '20' }, '2026-10')).toEqual({
      input: { kind: 'CUT', name: 'Comida', category: 'FOOD', cutPercent: 20 },
    })
    expect(toGoalInput({ ...form, kind: 'RESERVE', name: 'Reserva', months: '6' }, '2026-10')).toEqual({
      input: { kind: 'RESERVE', name: 'Reserva', reserveMonths: 6 },
    })
  })

  it('recusa o que a API recusaria', () => {
    const bad = (over: Partial<typeof form>) => 'error' in toGoalInput({ ...form, name: 'a', amount: '100', ...over }, '2026-10')
    expect(bad({ name: '  ' })).toBe(true)
    expect(bad({ name: 'a'.repeat(61) })).toBe(true)
    expect(bad({ amount: '0' })).toBe(true)
    expect(bad({ amount: '' })).toBe(true)
    expect(bad({ date: '2026-10' })).toBe(true) // mês atual não é futuro
    expect(bad({ kind: 'CUT', category: 'SALARY' })).toBe(true)
    expect(bad({ kind: 'CUT', percent: '95' })).toBe(true)
    expect(bad({ kind: 'CUT', percent: '10.5' })).toBe(true)
    expect(bad({ kind: 'RESERVE', months: '37' })).toBe(true)
    expect(bad({})).toBe(false)
  })
})

describe('progressRatio', () => {
  it('fica entre 0 e 1', () => {
    expect(progressRatio(2500, 10000)).toBe(0.25)
    expect(progressRatio(20000, 10000)).toBe(1)
    expect(progressRatio(-5, 10000)).toBe(0)
    expect(progressRatio(5, 0)).toBe(0)
  })
})

describe('goalStatus', () => {
  it('juntar valor: cabe, não cabe, sem histórico e batida', () => {
    expect(goalStatus(g({ fits: false }))).toMatchObject({ tone: 'critical' })
    expect(goalStatus(g({ fits: false })).text).toContain('Não cabe')
    expect(goalStatus(g({ fits: true, surplus: 3000 }))).toMatchObject({ tone: 'ok' })
    expect(goalStatus(g({ fits: null, surplus: null }))).toMatchObject({ tone: 'warning' })
    expect(goalStatus(g({ done: true }))).toEqual({ tone: 'ok', text: 'Meta batida.' })
  })

  it('reserva: cobertura em meses', () => {
    const r = g({ kind: 'RESERVE', reserveMonths: 6, target: 12000, current: 9000, coverage: 4.5 })
    expect(goalStatus(r).text).toContain('4,5 de 6 meses')
    expect(goalStatus(r).text).toContain('Faltam')
    expect(goalStatus({ ...r, done: true })).toMatchObject({ tone: 'ok' })
    expect(goalStatus({ ...r, target: 0 })).toMatchObject({ tone: 'warning' })
  })

  it('redução: dentro ou acima do teto', () => {
    const history = [{ month: '2026-09', total: 700, hit: false }, { month: '2026-10', total: 600, hit: true }]
    const c = g({ kind: 'CUT', target: 680, current: 600, history })
    expect(goalStatus(c)).toMatchObject({ tone: 'ok' })
    expect(goalStatus(c).text).toContain('1 de 2 meses')
    expect(goalStatus({ ...c, current: 700 })).toMatchObject({ tone: 'critical' })
  })
})
