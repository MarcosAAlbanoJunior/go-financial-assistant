import { describe, expect, it } from 'vitest'
import type { BudgetItem } from '../api/types'
import { cleanLabel, committedShare, itemsOf, splitOf } from './budget'

describe('splitOf', () => {
  it('soma as três classes', () => {
    expect(splitOf({ month: '2026-09', fixed: 100, installment: 50, variable: 350 })).toEqual({ fixed: 100, installment: 50, variable: 350, total: 500 })
  })

  it('mês sem dados vira zero', () => {
    expect(splitOf(undefined).total).toBe(0)
  })
})

describe('committedShare', () => {
  it('fixas mais parcelas sobre a renda', () => {
    expect(committedShare({ fixed: 1000, installment: 500, variable: 3000, total: 4500 }, 6000)).toBe(0.25)
  })

  it('sem renda não há o que comparar', () => {
    expect(committedShare({ fixed: 1000, installment: 0, variable: 0, total: 1000 }, 0)).toBeNull()
  })
})

describe('cleanLabel', () => {
  it('remove a marcação de parcela do fim', () => {
    expect(cleanLabel('ANUIDADE DIFERENCIADA (1/12)')).toBe('ANUIDADE DIFERENCIADA')
    expect(cleanLabel('Netflix')).toBe('Netflix')
    expect(cleanLabel('Loja (3/6) Centro')).toBe('Loja (3/6) Centro')
  })
})

describe('itemsOf', () => {
  it('filtra pela classe', () => {
    const items = [{ class: 'FIXED' }, { class: 'VARIABLE' }, { class: 'FIXED' }] as BudgetItem[]
    expect(itemsOf(items, 'FIXED')).toHaveLength(2)
  })
})
