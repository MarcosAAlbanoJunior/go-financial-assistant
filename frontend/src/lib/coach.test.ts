import { describe, expect, it } from 'vitest'
import type { CoachAction, CoachContext } from '../api/types'
import { byPriority, describeContext, priorityLabel, sentNames } from './coach'

const ctx: CoachContext = {
  mes: '2026-09', meses: ['2026-08', '2026-09'], receita_do_mes: 5000, despesa_do_mes: 3000,
  categorias: [{ nome: 'Alimentação', gasto_por_mes: [1, 2] }],
  sugestoes: [
    { id: 's1', tipo: 'conta fixa ou assinatura', nome: 'STREAMING', categoria: 'Lazer', valor_no_mes: 40, economia_possivel: 40 },
    { id: 's2', tipo: 'possível cobrança duplicada', nome: 'transferência para pessoa', categoria: 'Outros', valor_no_mes: 100, economia_possivel: 100 },
  ],
  metas: [{ id: 'm1', tipo: 'reserva de emergência', nome: 'Reserva', atual: 1, alvo: 2, atingida: false }],
}

const action = (id: string, priority: number): CoachAction => ({ suggestionId: id, priority, comment: '', question: '', suggestion: null })

describe('describeContext', () => {
  it('conta o que vai e lembra da regra de privacidade', () => {
    const lines = describeContext(ctx)
    expect(lines[0]).toContain('1 categoria nos últimos 2 meses')
    expect(lines[1]).toContain('2 sugestões')
    expect(lines[2]).toContain('1 meta,')
    expect(lines.at(-1)).toContain('transferência para pessoa')
  })

  it('singular e vazio', () => {
    const one = describeContext({ ...ctx, sugestoes: ctx.sugestoes.slice(0, 1), metas: [] })
    expect(one[1]).toContain('1 sugestão da')
    expect(one[2]).toBe('Nenhuma meta.')
    expect(describeContext({ ...ctx, sugestoes: [] })[1]).toBe('Nenhuma sugestão de corte da Revisão.')
  })
})

describe('sentNames, byPriority, priorityLabel', () => {
  it('lista os nomes de sugestões e metas', () => {
    expect(sentNames(ctx)).toEqual(['STREAMING', 'transferência para pessoa', 'Reserva'])
  })

  it('ordena pela prioridade sem mexer no original e mantém a ordem nos empates', () => {
    const list = [action('a', 3), action('b', 1), action('c', 3), action('d', 1)]
    expect(byPriority(list).map((a) => a.suggestionId)).toEqual(['b', 'd', 'a', 'c'])
    expect(list[0].suggestionId).toBe('a')
  })

  it('rótulos', () => {
    expect([1, 2, 3, 4, 5].map(priorityLabel)).toEqual(['Prioridade alta', 'Prioridade alta', 'Prioridade média', 'Prioridade baixa', 'Prioridade baixa'])
  })
})
