import { isMonth } from './months'

const brl = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })
const brlCompact = new Intl.NumberFormat('pt-BR', {
  style: 'currency',
  currency: 'BRL',
  notation: 'compact',
  minimumFractionDigits: 0,
  maximumFractionDigits: 1,
})
const brlWhole = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL', maximumFractionDigits: 0 })
const percent = new Intl.NumberFormat('pt-BR', { style: 'percent', maximumFractionDigits: 0 })

export const formatBRL = (value: number) => brl.format(value)

/** Sem centavos, para células apertadas. */
export const formatBRLWhole = (value: number) => brlWhole.format(value)

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

/** Quando o saldo foi atualizado: "hoje às 08:12", "ontem às 08:12" ou "03/09 às 08:12 (há 29 dias)". */
export function freshnessText(updatedAt: string, now: Date): string {
  const t = new Date(updatedAt)
  const time = t.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })
  const startOf = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
  const days = Math.round((startOf(now) - startOf(t)) / 86_400_000)
  if (days <= 0) return `hoje às ${time}`
  if (days === 1) return `ontem às ${time}`
  return `${t.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit' })} às ${time} (há ${days} dias)`
}
