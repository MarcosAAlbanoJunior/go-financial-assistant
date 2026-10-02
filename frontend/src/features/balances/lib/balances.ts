import { formatBRL } from '../../../shared/lib/format'
import type { BalanceCard, BalanceInstitution, Level } from '../api'

/** Distância mínima entre cores vizinhas da barra (ΔE em OKLab, escala 0-100). */
export const MIN_DELTA_E = 15

/** Reservas da série categórica do app (modo claro), na ordem fixa de uso. */
export const FALLBACK_COLORS = [
  { css: 'var(--series-1)', hex: '#2a78d6' },
  { css: 'var(--series-3)', hex: '#1baf7a' },
  { css: 'var(--series-4)', hex: '#eda100' },
  { css: 'var(--series-2)', hex: '#eb6834' },
]

/** Cinza neutro para banco sem cor de marca. */
export const NEUTRAL_COLOR = '#6b6a66'

const HEX = /^#[0-9a-f]{6}$/i

/** A cor vem da API e vai para um valor de CSS: só passa hexadecimal de 6 dígitos. */
export const safeHex = (color: string | null | undefined): string => (color && HEX.test(color) ? color.toLowerCase() : NEUTRAL_COLOR)

const channel = (hex: string, i: number) => parseInt(hex.slice(1 + i * 2, 3 + i * 2), 16)
const linear = (c: number) => {
  const v = c / 255
  return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4
}

function oklab(hex: string): [number, number, number] {
  const [r, g, b] = [0, 1, 2].map((i) => linear(channel(hex, i)))
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b)
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b)
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b)
  return [
    0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s,
    1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
    0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s,
  ]
}

/** Distância perceptiva entre duas cores (#rrggbb), em OKLab × 100. */
export function deltaE(a: string, b: string): number {
  const [l1, a1, b1] = oklab(a)
  const [l2, a2, b2] = oklab(b)
  return 100 * Math.hypot(l1 - l2, a1 - a2, b1 - b2)
}

function luminance(hex: string): number {
  const [r, g, b] = [0, 1, 2].map((i) => linear(channel(hex, i)))
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

/** Razão de contraste WCAG entre duas cores (#rrggbb), de 1 a 21. */
export function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

const INK_LIGHT = '#ffffff'
const INK_DARK = '#111111'

/** Tinta (branca ou quase preta) com o maior contraste sobre o fundo; no pior caso ainda é a melhor das duas. */
export const inkOn = (background: string): string =>
  contrast(INK_LIGHT, background) >= contrast(INK_DARK, background) ? INK_LIGHT : INK_DARK

export interface SliceColor {
  /** Valor de CSS para a fatia (hex da marca ou var(--series-N)). */
  css: string
  hex: string
  fromBrand: boolean
}

/**
 * Cor de cada fatia da barra. Usa a cor de marca enquanto cada par vizinho passar (ΔE ≥ 15); senão usa
 * a primeira cor da série categórica que se distinga da fatia anterior. A cor de marca segue na faixa,
 * no chip e no logo, e a legenda nunca depende de cor.
 */
export function sliceColors(brands: (string | null)[]): SliceColor[] {
  const out: SliceColor[] = []
  brands.forEach((brand, i) => {
    const prev = out[i - 1]
    const own = brand && HEX.test(brand) ? brand.toLowerCase() : null
    if (own && (!prev || deltaE(own, prev.hex) >= MIN_DELTA_E)) {
      out.push({ css: own, hex: own, fromBrand: true })
      return
    }
    const start = i % FALLBACK_COLORS.length
    for (let k = 0; k < FALLBACK_COLORS.length; k++) {
      const c = FALLBACK_COLORS[(start + k) % FALLBACK_COLORS.length]
      if (!prev || deltaE(c.hex, prev.hex) >= MIN_DELTA_E) {
        out.push({ css: c.css, hex: c.hex, fromBrand: false })
        return
      }
    }
    out.push({ css: FALLBACK_COLORS[start].css, hex: FALLBACK_COLORS[start].hex, fromBrand: false })
  })
  return out
}

/** Instituições que ganham fatia na barra: têm conta corrente com saldo positivo. */
export const sliced = (institutions: BalanceInstitution[]) => institutions.filter((i) => i.shareOfTotal > 0)

/** Inicial para o monograma (primeira letra ou número do nome). */
export function monogram(name: string): string {
  const m = name.match(/[\p{L}\p{N}]/u)
  return m ? m[0].toUpperCase() : '?'
}

export const HIDDEN_MONEY = 'R$ ••••'

export const money = (value: number, hidden: boolean) => (hidden ? HIDDEN_MONEY : formatBRL(value))

const dayMonth = (iso: string) => {
  const [, m, d] = iso.slice(0, 10).split('-')
  return `${d}/${m}`
}

/**
 * Texto do vencimento da fatura; null quando não há o que pagar, o banco não informou a data ou ela já passou
 * (o banco mantém o vencimento da fatura paga enquanto o valor devido é o da seguinte).
 */
export function dueText(card: Pick<BalanceCard, 'invoice' | 'dueDate' | 'daysToDue'>): string | null {
  if (card.invoice <= 0 || card.dueDate === null || card.daysToDue === null || card.daysToDue < 0) return null
  const date = dayMonth(card.dueDate)
  const days = card.daysToDue
  if (days === 0) return `vence hoje · ${date}`
  if (days === 1) return `vence amanhã · ${date}`
  return `vence ${date} · em ${days} dias`
}

export const LEVEL_LABEL: Record<Level, string> = { ok: '', warning: 'atenção', critical: 'crítico' }

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

/** Resumo de acessibilidade da barra de participação. */
export function shareLabel(institutions: BalanceInstitution[]): string {
  const parts = sliced(institutions).map((i) => `${i.name} ${Math.round(i.shareOfTotal * 100)}%`)
  return `Participação de cada banco no total em conta: ${parts.join(', ')}`
}
