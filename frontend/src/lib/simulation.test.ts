import { describe, expect, it } from 'vitest'
import type { Projection } from '../api/types'
import { costOf, parseScenarios, priceInstallment, scenarioByMonth, simulate, verdict, type Scenario } from './simulation'

const base: Scenario = {
  id: 'a', name: 'Carro', mode: 'installment', start: '2026-11', parcels: 3, payment: 1000,
  price: 0, down: 0, ratePct: 0, active: true,
}

const projection: Projection = {
  assumptions: { income: 6000, fixed: 1500, variable: 2500, basedOn: 3 },
  months: ['2026-10', '2026-11', '2026-12', '2027-01', '2027-02', '2027-03'].map((month, i) => ({
    month, fixed: 1500, variable: 2500, installment: i === 1 ? 200 : 0,
  })),
}
const premises = { income: 6000, fixed: 1500, variable: 2500 }

describe('priceInstallment', () => {
  it('calcula a parcela pela tabela Price', () => {
    // 10.000 a 1% ao mês em 12 vezes: 888,49
    expect(priceInstallment(10000, 1, 12)).toBeCloseTo(888.49, 2)
  })

  it('taxa zero divide o valor', () => {
    expect(priceInstallment(1200, 0, 12)).toBe(100)
  })

  it('entradas inválidas dão zero', () => {
    expect(priceInstallment(0, 1, 12)).toBe(0)
    expect(priceInstallment(1000, 1, 0)).toBe(0)
  })
})

describe('costOf', () => {
  it('modo parcela: total é a parcela vezes o número delas', () => {
    expect(costOf(base)).toEqual({ payment: 1000, down: 0, total: 3000, interest: 0 })
  })

  it('modo financiamento: entrada abate o financiado e os juros aparecem', () => {
    const c = costOf({ ...base, mode: 'financing', price: 12000, down: 2000, ratePct: 1, parcels: 12 })
    expect(c.payment).toBeCloseTo(888.49, 2)
    expect(c.down).toBe(2000)
    expect(c.total).toBeCloseTo(888.49 * 12 + 2000, 0)
    expect(c.interest).toBeCloseTo(c.total - 12000, 2)
  })

  it('entrada maior que o bem não gera parcela negativa', () => {
    expect(costOf({ ...base, mode: 'financing', price: 1000, down: 5000, ratePct: 1 }).payment).toBe(0)
  })
})

describe('scenarioByMonth', () => {
  const months = ['2026-10', '2026-11', '2026-12', '2027-01', '2027-02', '2027-03']

  it('cobra a parcela só nos meses do contrato', () => {
    expect(scenarioByMonth(base, months)).toEqual([0, 1000, 1000, 1000, 0, 0])
  })

  it('a entrada cai no primeiro mês', () => {
    const s = { ...base, mode: 'financing' as const, price: 5000, down: 2000, ratePct: 0, parcels: 3 }
    expect(scenarioByMonth(s, months)).toEqual([0, 3000, 1000, 1000, 0, 0])
  })

  it('início antes ou depois da janela', () => {
    expect(scenarioByMonth({ ...base, start: '2027-03', parcels: 12 }, months)).toEqual([0, 0, 0, 0, 0, 1000])
    expect(scenarioByMonth({ ...base, start: '2026-08', parcels: 3 }, months)).toEqual([1000, 0, 0, 0, 0, 0]) // ago, set e out: só out cai na janela
  })
})

describe('simulate + verdict', () => {
  it('compara o saldo sem e com o cenário', () => {
    const rows = simulate(projection, premises, [base])
    expect(rows[0]).toMatchObject({ base: 2000, scenario: 0, withScenario: 2000 })
    expect(rows[1]).toMatchObject({ base: 1800, scenario: 1000, withScenario: 800 }) // a parcela já conhecida de 200 entra na base
    expect(rows[4].withScenario).toBe(2000)

    const v = verdict(rows)!
    expect(v.worst).toEqual({ month: '2026-11', balance: 800 })
    expect(v.firstNegative).toBeNull()
    expect(v.peakShareOfIncome).toBeCloseTo(1000 / 6000)
    expect(v.averageLeft).toBeCloseTo((2000 + 800 + 1000 + 1000 + 2000 + 2000) / 6 + 0, 5) // 1466,67 menos 200 de parcela existente já na base
  })

  it('cenário desativado não pesa, e vários se somam', () => {
    expect(simulate(projection, premises, [{ ...base, active: false }])[1].scenario).toBe(0)
    expect(simulate(projection, premises, [base, { ...base, id: 'b', payment: 500 }])[1].scenario).toBe(1500)
  })

  it('aponta o primeiro mês negativo e conta os meses no vermelho', () => {
    const rows = simulate(projection, premises, [{ ...base, payment: 2500 }])
    const v = verdict(rows)!
    expect(v.firstNegative).toBe('2026-11')
    expect(v.negativeMonths).toBe(3)
    expect(v.negativeMonthsBase).toBe(0)
  })

  it('sem meses não há veredito', () => {
    expect(verdict([])).toBeNull()
  })
})

describe('parseScenarios', () => {
  it('lê cenários válidos', () => {
    expect(parseScenarios(JSON.stringify([base]))).toEqual([base])
  })

  it('descarta lixo, tipos errados e valores fora de faixa', () => {
    const bad = [
      { ...base, parcels: 0 }, { ...base, parcels: 1.5 }, { ...base, payment: -1 }, { ...base, start: '2026-13' },
      { ...base, mode: 'outro' }, { ...base, name: 'x'.repeat(61) }, { ...base, ratePct: 500 }, { ...base, active: 'sim' }, 'texto', null,
    ]
    expect(parseScenarios(JSON.stringify([...bad, base]))).toEqual([base])
  })

  it('JSON inválido ou formato errado vira lista vazia', () => {
    expect(parseScenarios('{{')).toEqual([])
    expect(parseScenarios('{"a":1}')).toEqual([])
    expect(parseScenarios(null)).toEqual([])
  })
})
