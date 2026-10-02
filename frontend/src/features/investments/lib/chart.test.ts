import { describe, expect, it } from 'vitest'
import { knownMonths } from '../../../shared/lib/chart'
import { toInvestmentRows, toPortfolioRows } from './chart'

describe('toInvestmentRows', () => {
  it('mantém o acumulado de cada mês', () => {
    const rows = toInvestmentRows([
      { month: '2026-08', applied: 100, redeemed: 0, cumulative: 100 },
      { month: '2026-09', applied: 0, redeemed: 30, cumulative: 70 },
    ])
    expect(rows[1]).toMatchObject({ label: 'set/26', redeemed: 30, cumulative: 70 })
  })
})

describe('toPortfolioRows', () => {
  it('separa saldo exato e estimado e liga as duas linhas no primeiro mês exato', () => {
    const rows = toPortfolioRows([
      { month: '2026-07', balance: null, estimated: true },
      { month: '2026-08', balance: 900, estimated: true },
      { month: '2026-09', balance: 1000, estimated: true },
      { month: '2026-10', balance: 1500.5, estimated: false },
      { month: '2026-11', balance: 1600, estimated: false },
    ])
    expect(rows.map((r) => r.balance)).toEqual([null, null, null, 1500.5, 1600])
    expect(rows.map((r) => r.estimated)).toEqual([null, 900, 1000, 1500.5, null])
    expect(knownMonths(rows, 'balance')).toBe(2)
    expect(knownMonths(rows, 'estimated')).toBe(3)
  })

  it('sem estimativa, só a série exata', () => {
    const rows = toPortfolioRows([{ month: '2026-10', balance: 10, estimated: false }])
    expect(rows[0]).toMatchObject({ balance: 10, estimated: null })
  })

  it('mês estimado sem valor continua sem valor e não liga nada', () => {
    const rows = toPortfolioRows([
      { month: '2026-09', balance: null, estimated: true },
      { month: '2026-10', balance: 5, estimated: false },
    ])
    expect(rows[1]).toMatchObject({ balance: 5, estimated: null })
  })
})
