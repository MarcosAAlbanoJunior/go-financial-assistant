import { describe, expect, it } from 'vitest'
import { groupPositions } from './portfolio'
import type { Position } from '../api'

const pos = (over: Partial<Position>): Position => ({
  id: crypto.randomUUID(),
  type: 'FIXED_INCOME',
  typeLabel: 'Renda fixa',
  subtype: 'CDB',
  name: 'CDB X',
  balance: 100,
  amount: 100,
  updatedAt: '2026-10-01T00:00:00Z',
  ...over,
})

describe('groupPositions', () => {
  it('soma posições do mesmo produto e conta os lotes', () => {
    const groups = groupPositions([pos({ balance: 100 }), pos({ balance: 50.5 }), pos({ name: 'Fundo Y', type: 'MUTUAL_FUND', balance: 400 })])
    expect(groups).toHaveLength(2)
    expect(groups[0]).toMatchObject({ name: 'Fundo Y', balance: 400, count: 1 })
    expect(groups[1]).toMatchObject({ name: 'CDB X', balance: 150.5, count: 2 })
  })

  it('esconde posições zeradas e não mistura tipos com o mesmo nome', () => {
    const groups = groupPositions([pos({ balance: 0 }), pos({ balance: -1 }), pos({ subtype: 'LCI', balance: 10 }), pos({ balance: 20 })])
    expect(groups.map((g) => [g.subtype, g.balance])).toEqual([['CDB', 20], ['LCI', 10]])
  })

  it('lista vazia', () => {
    expect(groupPositions([])).toEqual([])
  })
})
