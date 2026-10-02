// Um mês é sempre "AAAA-MM", o mesmo formato da API.

const MONTH_RE = /^(\d{4})-(0[1-9]|1[0-2])$/

export function isMonth(value: string | null | undefined): value is string {
  return value != null && MONTH_RE.test(value)
}

export function currentMonth(now = new Date()): string {
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
}

export function shiftMonth(month: string, delta: number): string {
  const [year, mon] = month.split('-').map(Number)
  const index = year * 12 + (mon - 1) + delta
  return `${Math.floor(index / 12)}-${String((index % 12) + 1).padStart(2, '0')}`
}
