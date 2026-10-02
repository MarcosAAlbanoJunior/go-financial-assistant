import { CopyPlus, Pin, Repeat, Sparkles, TrendingUp, type LucideIcon } from 'lucide-react'
import type { ReviewCandidate, ReviewKind } from '../api/types'
import { cleanLabel } from './budget'
import { formatBRL } from './format'

export interface KindMeta {
  label: string
  Icon: LucideIcon
}

export const KIND_META: Record<ReviewKind, KindMeta> = {
  INCREASE: { label: 'Aumentos', Icon: TrendingUp },
  FIXED: { label: 'Fixas e assinaturas', Icon: Pin },
  ANT: { label: 'Gasto formiga', Icon: Sparkles },
  DUPLICATE: { label: 'Possíveis duplicatas', Icon: CopyPlus },
  NEW: { label: 'Novas no mês', Icon: Repeat },
}

export const KIND_ORDER: ReviewKind[] = ['INCREASE', 'FIXED', 'ANT', 'DUPLICATE', 'NEW']

/**
 * Intensidade (0 a 1) de cada mês de uma categoria, relativa ao menor e ao maior valor da própria linha,
 * para destacar o que mudou e não só quem gasta mais (o valor exato fica escrito na célula).
 * Mês sem gasto é null (sem cor); linha sem variação fica toda no tom mais claro.
 */
export function heatLevels(values: number[]): (number | null)[] {
  const spent = values.filter((v) => v > 0)
  const min = Math.min(...spent)
  const max = Math.max(...spent)
  return values.map((v) => (v <= 0 ? null : max === min ? 0 : (v - min) / (max - min)))
}

/** Lista as sugestões do tipo escolhido; as dispensadas só aparecem se pedidas. */
export function visibleCandidates(list: ReviewCandidate[], kind: ReviewKind | 'ALL', showDismissed: boolean): ReviewCandidate[] {
  return list.filter((c) => (kind === 'ALL' || c.kind === kind) && c.dismissed === showDismissed)
}

/** Quantas sugestões não dispensadas há por tipo. */
export function countByKind(list: ReviewCandidate[]): Record<ReviewKind, number> {
  const out: Record<ReviewKind, number> = { INCREASE: 0, FIXED: 0, ANT: 0, DUPLICATE: 0, NEW: 0 }
  for (const c of list) if (!c.dismissed) out[c.kind]++
  return out
}

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

/** Frase que explica por que a sugestão apareceu. */
export function candidateDetail(c: ReviewCandidate): string {
  switch (c.kind) {
    case 'INCREASE':
      return `${formatBRL(c.amount)} no mês, contra a média de ${formatBRL(c.baseline)} dos meses anteriores`
    case 'FIXED':
      return `${formatBRL(c.amount)} por mês, há ${plural(c.months, 'mês', 'meses')}`
    case 'ANT':
      return `${plural(c.count, 'compra pequena', 'compras pequenas')} somando ${formatBRL(c.amount)} no mês`
    case 'DUPLICATE':
      return `${c.count} cobranças de ${formatBRL(c.amount)} em poucos dias`
    case 'NEW':
      return `${formatBRL(c.amount)}, e a conta não existia nos meses anteriores`
  }
}

/** Como a economia aparece: por mês e por ano, ou uma vez só nas avulsas. */
export function savingText(c: ReviewCandidate): string {
  return c.annual === null ? `${formatBRL(c.monthly)} uma vez só` : `${formatBRL(c.monthly)}/mês · ${formatBRL(c.annual)}/ano`
}

/** Nome para exibir: tira a marcação de parcela e, nos aumentos, usa o nome da categoria. */
export const candidateName = (c: ReviewCandidate) => (c.kind === 'INCREASE' ? c.categoryLabel : cleanLabel(c.label))

/** Atalho para as transações de uma categoria em um mês. */
export const transactionsLink = (month: string, category: string) =>
  `/transacoes?${new URLSearchParams({ mes: month, categoria: category, tipo: 'EXPENSE' })}`
