import { describe, expect, it } from 'vitest'
import { toInvestmentRows, toMonthRows } from './chart'
import { deltaPercent, formatBRL, formatBRLCompact, formatMonthLong, formatMonthShort, formatMonthTitle, formatPercent } from './format'

// Intl usa espaço sem quebra entre "R$" e o valor; normalizamos para comparar.
const plain = (s: string) => s.replace(/ /g, ' ')

describe('format', () => {
  it('formata reais', () => {
    expect(plain(formatBRL(1234.5))).toBe('R$ 1.234,50')
    expect(plain(formatBRL(-10))).toBe('-R$ 10,00')
  })

  it('formata reais compactos para eixos', () => {
    expect(plain(formatBRLCompact(1200))).toBe('R$ 1,2 mil')
    expect(plain(formatBRLCompact(0))).toBe('R$ 0')
  })

  it('formata meses sem deslocar por fuso', () => {
    expect(formatMonthLong('2026-09')).toBe('setembro de 2026')
    expect(formatMonthTitle('2026-09')).toBe('Setembro de 2026')
    expect(formatMonthShort('2026-01')).toBe('jan/26')
    expect(formatMonthShort('2026-12')).toBe('dez/26')
  })

  it('rejeita mês inválido', () => {
    expect(() => formatMonthLong('2026-13')).toThrow()
  })

  it('calcula variação e evita divisão por zero', () => {
    expect(deltaPercent(150, 100)).toBe(0.5)
    expect(deltaPercent(50, 100)).toBe(-0.5)
    expect(deltaPercent(10, 0)).toBeNull()
    expect(deltaPercent(-50, -100)).toBe(0.5)
    expect(formatPercent(0.5)).toBe('50%')
  })
})

describe('toMonthRows', () => {
  it('mantém a ordem e rotula os meses', () => {
    const rows = toMonthRows([
      { month: '2026-08', income: 0, expense: 455.04, applied: 0, redeemed: 0 },
      { month: '2026-09', income: 10, expense: 259.62, applied: 0, redeemed: 0 },
    ])
    expect(rows.map((r) => r.label)).toEqual(['ago/26', 'set/26'])
    expect(rows[1]).toMatchObject({ income: 10, expense: 259.62 })
  })
})

describe('toInvestmentRows', () => {
  it('mantém o acumulado de cada mês', () => {
    const rows = toInvestmentRows([
      { month: '2026-08', applied: 100, redeemed: 0, cumulative: 100 },
      { month: '2026-09', applied: 0, redeemed: 30, cumulative: 70 },
    ])
    expect(rows[1]).toMatchObject({ label: 'set/26', redeemed: 30, cumulative: 70 })
  })
})
