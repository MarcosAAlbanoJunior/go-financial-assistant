import { describe, expect, it } from 'vitest'
import { contrast, deltaE, dueText, HIDDEN_MONEY, inkOn, money, monogram, safeHex, shareLabel, sliceColors, sliced, NEUTRAL_COLOR } from './balances'
import type { BalanceInstitution } from '../api'

const plain = (s: string) => s.replace(/\u00a0/g, ' ')

const inst = (name: string, share: number, color: string | null = null): BalanceInstitution => ({
  id: name, name, color, logo: null, updatedAt: '2026-10-02T08:12:00Z', stale: false, total: share * 1000, shareOfTotal: share, accounts: [], cards: [],
})

describe('cores', () => {
  it('mede a distância em OKLab na escala da spec', () => {
    // Os valores medidos pelo validador de paleta: vermelho x laranja reprovam (13,4 < 15).
    expect(deltaE('#ec0000', '#ec7000')).toBeCloseTo(13.4, 1)
    expect(deltaE('#ec0000', '#8a05be')).toBeGreaterThan(15)
    expect(deltaE('#123456', '#123456')).toBe(0)
  })

  it('escolhe a tinta de maior contraste sobre a cor', () => {
    expect(inkOn('#ec0000')).toBe('#ffffff')
    expect(inkOn('#ec7000')).toBe('#111111')
    expect(inkOn('#ffffff')).toBe('#111111')
    expect(inkOn('#000000')).toBe('#ffffff')
    expect(contrast('#000000', '#ffffff')).toBeCloseTo(21, 0)
    for (const c of ['#ec0000', '#ec7000', '#8a05be', '#00a859', '#ffd400', '#0b3d91']) {
      expect(contrast(inkOn(c), c)).toBeGreaterThanOrEqual(4.5)
    }
  })

  it('aceita só hexadecimal de 6 dígitos', () => {
    expect(safeHex('#EC0000')).toBe('#ec0000')
    for (const bad of [null, undefined, '', 'red', '#fff', 'url(javascript:alert(1))', '#ec0000;x', '#gggggg']) {
      expect(safeHex(bad as string | null)).toBe(NEUTRAL_COLOR)
    }
  })
})

describe('sliceColors', () => {
  it('usa a cor de marca quando o par vizinho passa', () => {
    const out = sliceColors(['#ec0000', '#8a05be'])
    expect(out.map((c) => c.css)).toEqual(['#ec0000', '#8a05be'])
    expect(out.every((c) => c.fromBrand)).toBe(true)
  })

  it('troca pela série categórica quando o vizinho é parecido (vermelho e laranja)', () => {
    const [a, b] = sliceColors(['#ec0000', '#ec7000'])
    expect(a.css).toBe('#ec0000')
    expect(b.fromBrand).toBe(false)
    expect(b.css).toMatch(/^var\(--series-\d\)$/)
    expect(deltaE(a.hex, b.hex)).toBeGreaterThanOrEqual(15)
  })

  it('a reserva também precisa se distinguir do vizinho', () => {
    // Azul de marca anterior: a primeira reserva (azul) seria igual, então pula para a próxima.
    const [a, b] = sliceColors(['#2a78d6', '#2a78d7'])
    expect(deltaE(a.hex, b.hex)).toBeGreaterThanOrEqual(15)
  })

  it('banco sem cor usa a série; cores inválidas também', () => {
    expect(sliceColors([null, 'url(x)']).every((c) => !c.fromBrand)).toBe(true)
    expect(sliceColors([])).toEqual([])
  })
})

describe('textos', () => {
  const card = { invoice: 940, dueDate: '2026-10-04', daysToDue: 2 }
  it('vencimento', () => {
    expect(dueText(card)).toBe('vence 04/10 · em 2 dias')
    expect(dueText({ ...card, daysToDue: 1 })).toBe('vence amanhã · 04/10')
    expect(dueText({ ...card, daysToDue: 0 })).toBe('vence hoje · 04/10')
    // Data passada: a fatura já foi paga e o valor devido é o da seguinte; não mostra nem alerta.
    expect(dueText({ ...card, daysToDue: -1 })).toBeNull()
    expect(dueText({ ...card, daysToDue: -22 })).toBeNull()
  })

  it('sem fatura ou sem data, não há vencimento a mostrar', () => {
    expect(dueText({ ...card, invoice: 0 })).toBeNull()
    expect(dueText({ ...card, dueDate: null })).toBeNull()
    expect(dueText({ ...card, daysToDue: null })).toBeNull()
  })

  it('oculta valores', () => {
    expect(money(1234.5, true)).toBe(HIDDEN_MONEY)
    expect(plain(money(1234.5, false))).toBe('R$ 1.234,50')
  })

  it('monograma', () => {
    expect(monogram('itaú')).toBe('I')
    expect(monogram('  99 Pay')).toBe('9')
    expect(monogram('***')).toBe('?')
  })

  it('descreve a barra por escrito e ignora quem não tem fatia', () => {
    const list = [inst('Santander', 0.65), inst('Itaú', 0.35), inst('Roxo', 0)]
    expect(sliced(list).map((i) => i.name)).toEqual(['Santander', 'Itaú'])
    expect(shareLabel(list)).toBe('Participação de cada banco no total em conta: Santander 65%, Itaú 35%')
  })
})
