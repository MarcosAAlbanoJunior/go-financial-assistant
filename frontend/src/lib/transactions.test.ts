import { describe, expect, it } from 'vitest'
import { describeAmount, formatDay, monthFilter, toApiQuery } from './transactions'

const q = (s: string) => new URLSearchParams(s)
const plain = (s: string) => s.replace(/ /g, ' ')

describe('toApiQuery', () => {
  it('usa o mês atual por padrão e traduz os filtros', () => {
    expect(toApiQuery(q(''), '2026-10')).toBe('month=2026-10')
    expect(toApiQuery(q('mes=2026-09&tipo=EXPENSE&categoria=FOOD&forma=PIX&q=padaria&pagina=3'), '2026-10')).toBe(
      'month=2026-09&kind=EXPENSE&category=FOOD&payment_method=PIX&q=padaria&page=3',
    )
  })

  it('"todos" remove o mês e vazios são descartados', () => {
    expect(toApiQuery(q('mes=todos&tipo=&q=%20%20'), '2026-10')).toBe('')
  })

  it('ignora mês e página inválidos', () => {
    expect(toApiQuery(q('mes=lixo&pagina=-2'), '2026-10')).toBe('month=2026-10')
    expect(toApiQuery(q('pagina=1.5'), '2026-10')).toBe('month=2026-10')
  })

  it('codifica o texto da busca', () => {
    expect(toApiQuery(q('mes=todos&q=a%26b%3Dc'), '2026-10')).toBe('q=a%26b%3Dc')
  })
})

describe('monthFilter', () => {
  it('distingue "todos" de mês atual', () => {
    expect(monthFilter(q('mes=todos'), '2026-10')).toBeNull()
    expect(monthFilter(q(''), '2026-10')).toBe('2026-10')
  })
})

describe('describeAmount', () => {
  it('dá sinal e legenda por tipo', () => {
    expect(describeAmount({ kind: 'EXPENSE', amount: 45 })).toMatchObject({ tone: 'out', caption: 'Despesa' })
    expect(plain(describeAmount({ kind: 'EXPENSE', amount: 45 }).text)).toBe('-R$ 45,00')
    expect(plain(describeAmount({ kind: 'INCOME', amount: 10 }).text)).toBe('+R$ 10,00')
    expect(describeAmount({ kind: 'TRANSFER', transferDirection: 'IN', amount: 1 })).toMatchObject({ tone: 'neutral', caption: 'Resgate' })
    expect(describeAmount({ kind: 'TRANSFER', transferDirection: 'OUT', amount: 1 }).caption).toBe('Aplicação')
  })
})

describe('formatDay', () => {
  it('formata sem deslocar o dia', () => {
    expect(formatDay('2026-09-03')).toBe('03/09/2026')
  })
})
