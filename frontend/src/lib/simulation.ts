import type { Projection } from '../api/types'
import { shiftMonth } from './months'

export interface Scenario {
  id: string
  name: string
  /** "installment": parcela já conhecida. "financing": calcula a parcela pelo valor, taxa e prazo. */
  mode: 'installment' | 'financing'
  /** Mês da primeira parcela (AAAA-MM). */
  start: string
  parcels: number
  /** Valor de cada parcela (modo installment). */
  payment: number
  /** Modo financing: valor do bem, entrada paga no mês inicial e taxa de juros ao mês, em %. */
  price: number
  down: number
  ratePct: number
  active: boolean
}

export interface Premises {
  income: number
  fixed: number
  variable: number
}

/** Parcela pela tabela Price (parcelas iguais). Taxa zero divide o valor igualmente. */
export function priceInstallment(principal: number, ratePct: number, n: number): number {
  if (principal <= 0 || n <= 0) return 0
  const i = ratePct / 100
  if (i === 0) return principal / n
  return (principal * i) / (1 - Math.pow(1 + i, -n))
}

export interface ScenarioCost {
  /** Valor de cada parcela. */
  payment: number
  /** Entrada paga no primeiro mês (só no modo financing). */
  down: number
  /** Tudo o que será pago: parcelas mais entrada. */
  total: number
  /** Juros pagos: total menos o valor do bem (0 no modo installment, em que não se sabe). */
  interest: number
}

export function costOf(s: Scenario): ScenarioCost {
  if (s.mode === 'installment') {
    const total = s.payment * s.parcels
    return { payment: s.payment, down: 0, total, interest: 0 }
  }
  const financed = Math.max(0, s.price - s.down)
  const payment = priceInstallment(financed, s.ratePct, s.parcels)
  const total = payment * s.parcels + s.down
  return { payment, down: s.down, total, interest: Math.max(0, total - s.price) }
}

/** Quanto o cenário pesa em cada mês: a entrada no primeiro e a parcela nos `parcels` meses seguintes. */
export function scenarioByMonth(s: Scenario, months: string[]): number[] {
  const cost = costOf(s)
  return months.map((m) => {
    const offset = monthsBetween(s.start, m)
    if (offset < 0 || offset >= s.parcels) return 0
    return cost.payment + (offset === 0 ? cost.down : 0)
  })
}

function monthsBetween(from: string, to: string): number {
  const [fy, fm] = from.split('-').map(Number)
  const [ty, tm] = to.split('-').map(Number)
  return (ty - fy) * 12 + (tm - fm)
}

export interface Row {
  month: string
  income: number
  fixed: number
  installment: number
  variable: number
  scenario: number
  /** Saldo sem o cenário e com ele. */
  base: number
  withScenario: number
}

/** Junta a base da projeção, as premissas (editáveis) e os cenários ativos, mês a mês. */
export function simulate(p: Projection, premises: Premises, scenarios: Scenario[]): Row[] {
  const months = p.months.map((m) => m.month)
  const extra = months.map(() => 0)
  for (const s of scenarios.filter((x) => x.active)) {
    scenarioByMonth(s, months).forEach((v, i) => (extra[i] += v))
  }
  return p.months.map((m, i) => {
    const base = premises.income - premises.fixed - m.installment - premises.variable
    return {
      month: m.month,
      income: premises.income,
      fixed: premises.fixed,
      installment: m.installment,
      variable: premises.variable,
      scenario: extra[i],
      base,
      withScenario: base - extra[i],
    }
  })
}

export interface Verdict {
  worst: { month: string; balance: number }
  /** Sobra média por mês, com o cenário. */
  averageLeft: number
  /** Primeiro mês em que o saldo fica negativo (com o cenário), se algum. */
  firstNegative: string | null
  /** Quantos meses ficam negativos com o cenário e quantos já ficariam sem ele. */
  negativeMonths: number
  negativeMonthsBase: number
  /** Maior fração da renda tomada pelo cenário em um mês. */
  peakShareOfIncome: number
}

export function verdict(rows: Row[]): Verdict | null {
  if (rows.length === 0) return null
  const worst = rows.reduce((w, r) => (r.withScenario < w.withScenario ? r : w))
  const income = rows[0].income
  const peak = Math.max(...rows.map((r) => r.scenario))
  return {
    worst: { month: worst.month, balance: worst.withScenario },
    averageLeft: rows.reduce((s, r) => s + r.withScenario, 0) / rows.length,
    firstNegative: rows.find((r) => r.withScenario < 0)?.month ?? null,
    negativeMonths: rows.filter((r) => r.withScenario < 0).length,
    negativeMonthsBase: rows.filter((r) => r.base < 0).length,
    peakShareOfIncome: income > 0 ? peak / income : 0,
  }
}

/** Mês seguinte ao atual: o padrão para o início de um financiamento novo. */
export const defaultStart = (current: string) => shiftMonth(current, 1)

// --- Persistência local dos cenários (nada vai ao servidor) ---

const KEY = 'scenarios'
const finite = (v: unknown, min: number, max: number): v is number => typeof v === 'number' && Number.isFinite(v) && v >= min && v <= max

/** Lê só o que tem o formato esperado; qualquer dado estranho no armazenamento é descartado. */
export function parseScenarios(raw: string | null): Scenario[] {
  try {
    const data: unknown = raw ? JSON.parse(raw) : []
    if (!Array.isArray(data)) return []
    return data.flatMap((d): Scenario[] => {
      if (typeof d !== 'object' || d === null) return []
      const s = d as Partial<Scenario>
      const ok =
        typeof s.id === 'string' && typeof s.name === 'string' && s.name.length <= 60 &&
        (s.mode === 'installment' || s.mode === 'financing') && typeof s.start === 'string' && /^\d{4}-(0[1-9]|1[0-2])$/.test(s.start) &&
        Number.isInteger(s.parcels) && finite(s.parcels, 1, 480) && finite(s.payment, 0, 1e9) && finite(s.price, 0, 1e10) &&
        finite(s.down, 0, 1e10) && finite(s.ratePct, 0, 100) && typeof s.active === 'boolean'
      return ok ? [s as Scenario] : []
    })
  } catch {
    return []
  }
}

export function loadScenarios(): Scenario[] {
  try {
    return parseScenarios(localStorage.getItem(KEY))
  } catch {
    return []
  }
}

export function saveScenarios(list: Scenario[]) {
  try {
    localStorage.setItem(KEY, JSON.stringify(list))
  } catch {
    // sem armazenamento, os cenários valem só nesta sessão
  }
}

// --- Premissas editadas (também só no navegador) ---

const PREMISES_KEY = 'projection-premises'
const FIELDS = ['income', 'fixed', 'variable'] as const

/** Lê só números finitos e não negativos das premissas editadas; o resto é descartado. */
export function parsePremises(raw: string | null): Partial<Premises> {
  try {
    const data: unknown = raw ? JSON.parse(raw) : {}
    if (typeof data !== 'object' || data === null || Array.isArray(data)) return {}
    const out: Partial<Premises> = {}
    for (const f of FIELDS) {
      const v = (data as Record<string, unknown>)[f]
      if (typeof v === 'number' && Number.isFinite(v) && v >= 0 && v <= 1e9) out[f] = v
    }
    return out
  } catch {
    return {}
  }
}

export function loadPremises(): Partial<Premises> {
  try {
    return parsePremises(localStorage.getItem(PREMISES_KEY))
  } catch {
    return {}
  }
}

export function savePremises(p: Partial<Premises>) {
  try {
    if (Object.keys(p).length === 0) localStorage.removeItem(PREMISES_KEY)
    else localStorage.setItem(PREMISES_KEY, JSON.stringify(p))
  } catch {
    // sem armazenamento, a edição vale só nesta sessão
  }
}
