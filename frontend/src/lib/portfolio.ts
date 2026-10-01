import type { Position } from '../api/types'

export interface PositionGroup {
  key: string
  name: string
  typeLabel: string
  subtype: string
  balance: number
  /** Quantas posições (lotes) o grupo reúne. */
  count: number
}

/**
 * Bancos como o Itaú devolvem uma posição por aplicação, com o mesmo nome (ex.: dezenas de
 * "CDB - ITAU UNIBANCO S.A."). Agrupa por produto e esconde as posições zeradas.
 */
export function groupPositions(positions: Position[]): PositionGroup[] {
  const groups = new Map<string, PositionGroup>()
  for (const p of positions) {
    if (p.balance <= 0) continue
    const key = `${p.type}|${p.subtype}|${p.name}`
    const g = groups.get(key)
    if (g) {
      g.balance += p.balance
      g.count += 1
    } else {
      groups.set(key, { key, name: p.name, typeLabel: p.typeLabel, subtype: p.subtype, balance: p.balance, count: 1 })
    }
  }
  return [...groups.values()].sort((a, b) => b.balance - a.balance)
}
