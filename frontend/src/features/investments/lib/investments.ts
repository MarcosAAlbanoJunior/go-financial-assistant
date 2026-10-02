import type { InvestmentMonth } from '../api'

export interface InvestmentSummary {
  applied: number
  redeemed: number
  net: number
  /** Líquido acumulado ao fim do período (vem da API, que soma desde o primeiro lançamento). */
  cumulative: number
}

export function summarizeInvestments(months: InvestmentMonth[]): InvestmentSummary {
  const applied = months.reduce((s, m) => s + m.applied, 0)
  const redeemed = months.reduce((s, m) => s + m.redeemed, 0)
  return { applied, redeemed, net: applied - redeemed, cumulative: months.at(-1)?.cumulative ?? 0 }
}
