import { formatBRL, formatMonthLong } from '../../../shared/lib/format'
import type { Tone } from '../../../shared/lib/tone'
import type { ReviewKind, SavingDecision } from '../api'

/** Só as sugestões que já provaram se repetir aceitam "cancelei" (aumento, duplicata e conta nova podem ser avulsos). */
export const canDecide = (kind: ReviewKind) => kind === 'FIXED' || kind === 'ANT'

const months = (n: number) => `${n} ${n === 1 ? 'mês' : 'meses'}`

/** A situação da decisão em uma frase; o tom só reforça (ícone e texto sempre acompanham). */
export function savingStatus(d: SavingDecision): { tone: Tone; text: string } {
  switch (d.status) {
    case 'CONFIRMED':
      return { tone: 'ok', text: `Confirmada: ${months(d.monthsConfirmed)} sem a cobrança, ${formatBRL(d.realized)} a menos até agora.` }
    case 'RETURNED':
      return { tone: 'critical', text: `A cobrança voltou (${formatBRL(d.returned)}). Confira se o cancelamento valeu.` }
    case 'PENDING':
      return { tone: 'warning', text: `Aguardando o primeiro mês fechado depois de ${formatMonthLong(d.month)}.` }
  }
}
