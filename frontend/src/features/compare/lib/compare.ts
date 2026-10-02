import type { BreakdownItem } from '../../../shared/api/totals'

export interface CompareRow {
  key: string
  label: string
  current: number
  previous: number
}

/** Junta dois meses pela chave do grupo (um grupo pode existir só em um deles), do maior para o menor. */
export function mergeBreakdowns(current: BreakdownItem[], previous: BreakdownItem[]): CompareRow[] {
  const rows = new Map<string, CompareRow>()
  for (const i of previous) rows.set(i.key, { key: i.key, label: i.label, current: 0, previous: i.total })
  for (const i of current) {
    const row = rows.get(i.key)
    if (row) row.current = i.total
    else rows.set(i.key, { key: i.key, label: i.label, current: i.total, previous: 0 })
  }
  return [...rows.values()].sort((a, b) => Math.max(b.current, b.previous) - Math.max(a.current, a.previous))
}
