import { isMonth } from './months'

const brl = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })
const brlCompact = new Intl.NumberFormat('pt-BR', {
  style: 'currency',
  currency: 'BRL',
  notation: 'compact',
  minimumFractionDigits: 0,
  maximumFractionDigits: 1,
})
const percent = new Intl.NumberFormat('pt-BR', { style: 'percent', maximumFractionDigits: 0 })

export const formatBRL = (value: number) => brl.format(value)

/** Para eixos de gráfico, ex.: "R$ 1,2 mil". */
export const formatBRLCompact = (value: number) => brlCompact.format(value)

function monthDate(month: string): Date {
  if (!isMonth(month)) throw new Error(`mês inválido: ${month}`)
  const [year, mon] = month.split('-').map(Number)
  return new Date(Date.UTC(year, mon - 1, 1))
}

/** "setembro de 2026" */
export const formatMonthLong = (month: string) =>
  monthDate(month).toLocaleDateString('pt-BR', { month: 'long', year: 'numeric', timeZone: 'UTC' })

/** "Setembro de 2026", para títulos. */
export function formatMonthTitle(month: string): string {
  const long = formatMonthLong(month)
  return long.charAt(0).toUpperCase() + long.slice(1)
}

/** "set/26", para eixos. */
export function formatMonthShort(month: string): string {
  const d = monthDate(month)
  const name = d.toLocaleDateString('pt-BR', { month: 'short', timeZone: 'UTC' }).replace('.', '')
  return `${name}/${String(d.getUTCFullYear()).slice(2)}`
}

/** Variação em relação ao mês anterior; null quando não há base de comparação (anterior zerado). */
export function deltaPercent(current: number, previous: number): number | null {
  return previous === 0 ? null : (current - previous) / Math.abs(previous)
}

export const formatPercent = (value: number) => percent.format(value)
