import { describe, expect, it } from 'vitest'
import { mergeBreakdowns } from './compare'

const item = (key: string, total: number) => ({ key, label: key.toLowerCase(), total })

describe('mergeBreakdowns', () => {
  it('une os meses pela chave, inclusive grupos que existem em só um deles', () => {
    const rows = mergeBreakdowns([item('FOOD', 50), item('NEW', 10)], [item('FOOD', 80), item('GONE', 30)])
    expect(rows).toEqual([
      { key: 'FOOD', label: 'food', current: 50, previous: 80 },
      { key: 'GONE', label: 'gone', current: 0, previous: 30 },
      { key: 'NEW', label: 'new', current: 10, previous: 0 },
    ])
  })

  it('ordena pelo maior valor entre os dois meses', () => {
    const rows = mergeBreakdowns([item('A', 5), item('B', 100)], [item('A', 200), item('B', 1)])
    expect(rows.map((r) => r.key)).toEqual(['A', 'B'])
  })

  it('aceita listas vazias', () => {
    expect(mergeBreakdowns([], [])).toEqual([])
    expect(mergeBreakdowns([item('A', 1)], [])).toHaveLength(1)
  })
})
