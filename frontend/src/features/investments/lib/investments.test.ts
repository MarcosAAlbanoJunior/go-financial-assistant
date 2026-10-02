import { describe, expect, it } from 'vitest'
import { summarizeInvestments } from './investments'

describe('summarizeInvestments', () => {
  it('soma o período e usa o acumulado do último mês', () => {
    const s = summarizeInvestments([
      { month: '2026-08', applied: 100, redeemed: 0, cumulative: 500 },
      { month: '2026-09', applied: 50, redeemed: 30, cumulative: 520 },
    ])
    expect(s).toEqual({ applied: 150, redeemed: 30, net: 120, cumulative: 520 })
  })

  it('aceita período sem lançamentos', () => {
    expect(summarizeInvestments([])).toEqual({ applied: 0, redeemed: 0, net: 0, cumulative: 0 })
  })

  it('líquido negativo quando resgata mais do que aplica', () => {
    expect(summarizeInvestments([{ month: '2026-09', applied: 10, redeemed: 40, cumulative: 0 }]).net).toBe(-30)
  })
})
