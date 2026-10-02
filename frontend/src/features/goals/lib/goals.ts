import type { Tone } from '../../../shared/lib/tone'
import { PiggyBank, Scissors, ShieldCheck, type LucideIcon } from 'lucide-react'
import { CATEGORIES } from '../../../shared/lib/labels'
import { formatBRL, formatMonthLong } from '../../../shared/lib/format'
import { isMonth } from '../../../shared/lib/months'
import type { Goal, GoalInput, GoalKind } from '../api'

export interface GoalKindMeta {
  label: string
  hint: string
  Icon: LucideIcon
}

export const GOAL_KINDS: Record<GoalKind, GoalKindMeta> = {
  SAVE: { label: 'Juntar um valor', hint: 'Quanto guardar até uma data', Icon: PiggyBank },
  CUT: { label: 'Reduzir uma categoria', hint: 'Gastar X% menos do que a média dos meses anteriores', Icon: Scissors },
  RESERVE: { label: 'Reserva de emergência', hint: 'Manter N meses de despesas fixas', Icon: ShieldCheck },
}

export const GOAL_ORDER: GoalKind[] = ['SAVE', 'CUT', 'RESERVE']

/** Categorias de despesa que aceitam meta de redução (renda e investimento ficam de fora). */
export const CUT_CATEGORIES = CATEGORIES.filter((c) => c.value !== 'INVESTMENT' && c.value !== 'SALARY')

/** Valores do formulário, todos como texto. */
export interface GoalForm {
  kind: GoalKind
  name: string
  amount: string
  date: string
  category: string
  percent: string
  months: string
}

export const emptyGoalForm = (nextMonth: string): GoalForm => ({
  kind: 'SAVE',
  name: '',
  amount: '',
  date: nextMonth,
  category: 'FOOD',
  percent: '15',
  months: '6',
})

/** Valida o formulário (os mesmos limites da API) e monta o corpo; ou devolve a mensagem do erro. */
export function toGoalInput(f: GoalForm, thisMonth: string): { input: GoalInput } | { error: string } {
  const name = f.name.trim()
  if (name.length < 1 || name.length > 60) return { error: 'Dê um nome de até 60 caracteres.' }
  switch (f.kind) {
    case 'SAVE': {
      const amount = Number(f.amount)
      if (!(amount > 0) || amount > 1e9) return { error: 'Informe um valor maior que zero.' }
      if (!isMonth(f.date) || f.date <= thisMonth) return { error: 'A data precisa ser um mês futuro.' }
      return { input: { kind: 'SAVE', name, targetAmount: amount, targetDate: f.date } }
    }
    case 'CUT': {
      const percent = Number(f.percent)
      if (!CUT_CATEGORIES.some((c) => c.value === f.category)) return { error: 'Escolha uma categoria.' }
      if (!Number.isInteger(percent) || percent < 1 || percent > 90) return { error: 'A redução vai de 1% a 90%.' }
      return { input: { kind: 'CUT', name, category: f.category, cutPercent: percent } }
    }
    case 'RESERVE': {
      const months = Number(f.months)
      if (!Number.isInteger(months) || months < 1 || months > 36) return { error: 'A reserva vai de 1 a 36 meses.' }
      return { input: { kind: 'RESERVE', name, reserveMonths: months } }
    }
  }
}

/** Parte do alvo já atingida, de 0 a 1. */
export const progressRatio = (current: number, target: number) => (target > 0 ? Math.min(1, Math.max(0, current / target)) : 0)


export interface GoalStatus {
  tone: Tone
  text: string
}

const months = (n: number) => `${n} ${n === 1 ? 'mês' : 'meses'}`

/** O veredito da meta em uma frase; o tom só reforça (o ícone e o texto sempre acompanham). */
export function goalStatus(g: Goal): GoalStatus {
  switch (g.kind) {
    case 'SAVE': {
      if (g.done) return { tone: 'ok', text: 'Meta batida.' }
      const when = g.monthsLeft === 0 ? 'neste mês' : `em ${months(g.monthsLeft)}`
      const need = `Guardar ${formatBRL(g.perMonth)} por mês ${g.monthsLeft === 0 ? 'agora' : `(${when}, até ${formatMonthLong(g.targetDate ?? '')})`}.`
      if (g.fits === null || g.surplus === null) return { tone: 'warning', text: `${need} Sem histórico para saber se cabe no orçamento.` }
      return g.fits
        ? { tone: 'ok', text: `${need} Cabe no orçamento: a sobra projetada é de ${formatBRL(g.surplus)} por mês.` }
        : { tone: 'critical', text: `${need} Não cabe: a sobra projetada é de ${formatBRL(Math.max(0, g.surplus))} por mês.` }
    }
    case 'RESERVE': {
      if (g.target <= 0) return { tone: 'warning', text: 'Sem contas fixas no histórico para calcular o alvo.' }
      const cover = `O patrimônio cobre ${g.coverage.toFixed(1).replace('.', ',')} de ${months(g.reserveMonths)} de despesas fixas.`
      return g.done
        ? { tone: 'ok', text: `Reserva completa. ${cover}` }
        : { tone: 'warning', text: `Faltam ${formatBRL(g.target - g.current)}. ${cover}` }
    }
    case 'CUT': {
      // O mês atual ainda está aberto: só os meses fechados contam como "dentro do teto".
      const closed = g.history.slice(0, -1)
      const hit = closed.filter((h) => h.hit).length
      const within =
        closed.length === 0
          ? 'Ainda não há mês fechado desde que a meta existe.'
          : `Dentro do teto em ${hit} de ${months(closed.length)} fechados.`
      const month = `Este mês: ${formatBRL(g.current)} de ${formatBRL(g.target)}.`
      if (g.current > g.target) {
        return {
          tone: 'critical',
          text: `Este mês já passou ${formatBRL(g.current - g.target)} do teto de ${formatBRL(g.target)}. ${within}`,
        }
      }
      if (g.projected === null) {
        return {
          tone: 'ok',
          text: `${month} Estamos no dia ${g.dayOfMonth} de ${g.daysInMonth}: o ritmo aparece a partir do dia 7. ${within}`,
        }
      }
      return g.projected > g.target
        ? { tone: 'warning', text: `${month} No ritmo atual o mês fecha em ${formatBRL(g.projected)}, acima do teto. ${within}` }
        : { tone: 'ok', text: `${month} No ritmo atual o mês fecha em ${formatBRL(g.projected)}, dentro do teto. ${within}` }
    }
  }
}
