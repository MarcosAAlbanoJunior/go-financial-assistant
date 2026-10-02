import { describe, expect, it } from 'vitest'
import { currentMonth, isMonth, shiftMonth } from './months'

describe('months', () => {
  it('valida o formato AAAA-MM', () => {
    expect(isMonth('2026-09')).toBe(true)
    for (const bad of ['2026-9', '2026-13', '2026-00', 'setembro', '', null, undefined]) {
      expect(isMonth(bad)).toBe(false)
    }
  })

  it('usa o mês local, não UTC', () => {
    expect(currentMonth(new Date(2026, 0, 31, 23, 59))).toBe('2026-01')
    expect(currentMonth(new Date(2026, 11, 1))).toBe('2026-12')
  })

  it('desloca meses atravessando o ano', () => {
    expect(shiftMonth('2026-01', -1)).toBe('2025-12')
    expect(shiftMonth('2026-12', 1)).toBe('2027-01')
    expect(shiftMonth('2026-09', -11)).toBe('2025-10')
    expect(shiftMonth('2026-09', 0)).toBe('2026-09')
    expect(shiftMonth('2026-03', -15)).toBe('2024-12')
  })
})
