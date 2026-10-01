// Formatos das respostas da API do app Go (backend/internal/infra/http/api.go).

export interface Totals {
  month: string
  income: number
  expense: number
  applied: number
  redeemed: number
}

export interface Summary {
  month: string
  current: Totals
  previous: Totals
  /** Soma das contas correntes; null quando não há conta sincronizada. */
  bankBalance: number | null
}
