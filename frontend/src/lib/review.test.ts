import { describe, expect, it } from 'vitest'
import type { ReviewCandidate } from '../api/types'
import { candidateDetail, candidateName, countByKind, heatLevels, savingText, transactionsLink, visibleCandidates } from './review'

const base: ReviewCandidate = {
  kind: 'FIXED', key: 'streaming', label: 'STREAMING (1/12)', category: 'ENTERTAINMENT', categoryLabel: 'Lazer',
  monthly: 40, annual: 480, amount: 40, baseline: 0, count: 1, months: 6, dismissed: false,
}
const c = (over: Partial<ReviewCandidate>): ReviewCandidate => ({ ...base, ...over })

describe('heatLevels', () => {
  it('escala pelo menor e maior valor da linha, deixando o mês sem gasto sem cor', () => {
    expect(heatLevels([0, 100, 200, 150])).toEqual([null, 0, 1, 0.5])
  })

  it('linha sem variação fica toda no tom mais claro', () => {
    expect(heatLevels([50, 50, 0, 50])).toEqual([0, 0, null, 0])
  })

  it('linha vazia não quebra', () => {
    expect(heatLevels([0, 0])).toEqual([null, null])
  })
})

describe('visibleCandidates e countByKind', () => {
  const list = [c({}), c({ kind: 'ANT', key: 'a' }), c({ kind: 'ANT', key: 'b', dismissed: true })]

  it('filtra por tipo e separa as dispensadas', () => {
    expect(visibleCandidates(list, 'ALL', false)).toHaveLength(2)
    expect(visibleCandidates(list, 'ANT', false).map((x) => x.key)).toEqual(['a'])
    expect(visibleCandidates(list, 'ALL', true).map((x) => x.key)).toEqual(['b'])
  })

  it('conta só as não dispensadas', () => {
    expect(countByKind(list)).toEqual({ INCREASE: 0, FIXED: 1, ANT: 1, DUPLICATE: 0, NEW: 0 })
  })
})

describe('textos', () => {
  it('economia mensal e anual, ou única nas avulsas', () => {
    expect(savingText(base)).toMatch(/^R\$\s40,00\/mês · R\$\s480,00\/ano$/)
    expect(savingText(c({ annual: null, monthly: 250 }))).toMatch(/^R\$\s250,00 uma vez só$/)
  })

  it('detalhe no singular e no plural', () => {
    expect(candidateDetail(c({ months: 1 }))).toContain('há 1 mês')
    expect(candidateDetail(c({ months: 6 }))).toContain('há 6 meses')
    expect(candidateDetail(c({ kind: 'ANT', count: 5, amount: 90 }))).toContain('5 compras pequenas')
  })

  it('nome: categoria nos aumentos, descrição limpa nas demais', () => {
    expect(candidateName(c({ kind: 'INCREASE', label: '' }))).toBe('Lazer')
    expect(candidateName(base)).toBe('STREAMING')
  })

  it('link para as despesas da categoria no mês', () => {
    expect(transactionsLink('2026-09', 'FOOD')).toBe('/transacoes?mes=2026-09&categoria=FOOD&tipo=EXPENSE')
  })
})
