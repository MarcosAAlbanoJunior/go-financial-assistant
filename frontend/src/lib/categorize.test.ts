import { describe, expect, it } from 'vitest'
import type { UncategorizedGroup } from '../api/types'
import { groupSummary, pendingChoices, totalOf, withSuggestions } from './categorize'

const g = (key: string, over: Partial<UncategorizedGroup> = {}): UncategorizedGroup => ({
  key,
  label: key,
  count: 8,
  total: 400,
  last: '2026-09',
  transfer: false,
  ...over,
})

describe('groupSummary', () => {
  it('resume a conta no singular e no plural', () => {
    expect(groupSummary(g('a'))).toMatch(/^8 lançamentos · R\$\s400,00 · último em set\/26$/)
    expect(groupSummary(g('a', { count: 1 }))).toContain('1 lançamento ·')
  })
})

describe('pendingChoices e withSuggestions', () => {
  const groups = [g('a'), g('b'), g('c')]

  it('só aplica o que foi escolhido e ainda está na lista', () => {
    expect(pendingChoices({ a: 'FOOD', c: 'BILLS', gone: 'FOOD' }, groups)).toEqual([
      { key: 'a', category: 'FOOD' },
      { key: 'c', category: 'BILLS' },
    ])
    expect(pendingChoices({}, groups)).toEqual([])
  })

  it('a sugestão da IA não sobrescreve a escolha manual', () => {
    expect(
      withSuggestions({ a: 'HOUSING' }, [
        { key: 'a', category: 'FOOD' },
        { key: 'b', category: 'MARKET' },
      ]),
    ).toEqual({ a: 'HOUSING', b: 'MARKET' })
  })

  it('soma o total', () => {
    expect(totalOf(groups)).toBe(1200)
    expect(totalOf([])).toBe(0)
  })
})
