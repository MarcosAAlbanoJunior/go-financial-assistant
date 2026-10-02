import type { CoachAction, CoachContext } from '../api/types'

/** Resumo legível do que será enviado (o JSON completo também fica à vista). */
export function describeContext(c: CoachContext): string[] {
  const lines = [
    `Mês analisado: ${c.mes}, com receita e despesa do mês (totais) e o gasto de ${c.categorias.length} ${c.categorias.length === 1 ? 'categoria' : 'categorias'} nos últimos ${c.meses.length} meses.`,
    c.sugestoes.length === 0
      ? 'Nenhuma sugestão de corte da Revisão.'
      : `${c.sugestoes.length} ${c.sugestoes.length === 1 ? 'sugestão' : 'sugestões'} da Revisão, com o nome do serviço ou da conta, a categoria e os valores.`,
    c.metas.length === 0 ? 'Nenhuma meta.' : `${c.metas.length} ${c.metas.length === 1 ? 'meta' : 'metas'}, com o nome que você deu e o progresso.`,
    'Pix, TED e transferências vão como "transferência para pessoa". Não vão CPF, nome do titular nem número de conta.',
  ]
  return lines
}

/** Nomes que serão enviados, para a pessoa conferir antes (sugestões e metas). */
export const sentNames = (c: CoachContext): string[] => [...c.sugestoes.map((s) => s.nome), ...c.metas.map((m) => m.nome)]

/** Da prioridade 1 (mais importante) para a 5; empates mantêm a ordem recebida. */
export const byPriority = (actions: CoachAction[]): CoachAction[] => [...actions].sort((a, b) => a.priority - b.priority)

/** "Prioridade alta/média/baixa" a partir da escala de 1 a 5. */
export const priorityLabel = (p: number): string => (p <= 2 ? 'Prioridade alta' : p === 3 ? 'Prioridade média' : 'Prioridade baixa')
