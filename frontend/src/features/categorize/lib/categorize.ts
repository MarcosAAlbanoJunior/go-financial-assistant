import { formatBRL, formatMonthShort } from '../../../shared/lib/format'
import type { UncategorizedGroup } from '../api'

/** "8 lançamentos · R$ 400,00 · último em set/26" */
export function groupSummary(g: UncategorizedGroup): string {
  return `${g.count} ${g.count === 1 ? 'lançamento' : 'lançamentos'} · ${formatBRL(g.total)} · último em ${formatMonthShort(g.last)}`
}

/** As escolhas que dá para aplicar: só de contas que ainda estão na lista e com categoria escolhida. */
export function pendingChoices(choice: Record<string, string>, groups: UncategorizedGroup[]): { key: string; category: string }[] {
  return groups.filter((g) => choice[g.key]).map((g) => ({ key: g.key, category: choice[g.key] }))
}

/** Junta as sugestões da IA às escolhas atuais: o que você já escolheu à mão não é sobrescrito. */
export function withSuggestions(choice: Record<string, string>, suggestions: { key: string; category: string }[]): Record<string, string> {
  const next = { ...choice }
  for (const s of suggestions) if (!next[s.key]) next[s.key] = s.category
  return next
}

/** Total das despesas em Outros que a lista cobre. */
export const totalOf = (groups: UncategorizedGroup[]) => groups.reduce((sum, g) => sum + g.total, 0)
