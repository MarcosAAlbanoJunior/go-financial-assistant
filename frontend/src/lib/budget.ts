import { Activity, Layers, Pin, type LucideIcon } from 'lucide-react'
import type { BudgetItem, BudgetMonth, ExpenseClass } from '../api/types'

export interface ClassMeta {
  label: string
  /** Rótulo no singular, para frases como "Marcar como ...". */
  noun: string
  color: string
  Icon: LucideIcon
}

export const CLASS_META: Record<ExpenseClass, ClassMeta> = {
  FIXED: { label: 'Contas fixas', noun: 'fixa', color: 'var(--series-1)', Icon: Pin },
  INSTALLMENT: { label: 'Parceladas', noun: 'parcelada', color: 'var(--series-2)', Icon: Layers },
  VARIABLE: { label: 'Variáveis', noun: 'variável', color: 'var(--series-3)', Icon: Activity },
}

export const CLASS_ORDER: ExpenseClass[] = ['FIXED', 'INSTALLMENT', 'VARIABLE']

export interface Split {
  fixed: number
  installment: number
  variable: number
  total: number
}

export function splitOf(m: BudgetMonth | undefined): Split {
  const fixed = m?.fixed ?? 0
  const installment = m?.installment ?? 0
  const variable = m?.variable ?? 0
  return { fixed, installment, variable, total: fixed + installment + variable }
}

export const itemsOf = (items: BudgetItem[], cls: ExpenseClass) => items.filter((i) => i.class === cls)

/** Parte da renda já comprometida com fixas e parcelas; null sem renda para comparar. */
export function committedShare(split: Split, income: number): number | null {
  return income > 0 ? (split.fixed + split.installment) / income : null
}

/** Tira o "(2/12)" do fim da descrição para o nome ficar limpo. */
export const cleanLabel = (label: string) => label.replace(/\s*\(\s*\d+\s*\/\s*\d+\s*\)\s*$/, '').trim()
