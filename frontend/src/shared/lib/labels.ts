// Valores aceitos pela API (domain/purchase.go) e seus rótulos nos filtros.
export const KINDS = [
  { value: 'EXPENSE', label: 'Despesa' },
  { value: 'INCOME', label: 'Receita' },
  { value: 'TRANSFER', label: 'Investimento' },
]

export const CATEGORIES = [
  { value: 'FOOD', label: 'Alimentação' },
  { value: 'MARKET', label: 'Mercado' },
  { value: 'TRANSPORT', label: 'Transporte' },
  { value: 'HEALTH', label: 'Saúde' },
  { value: 'ENTERTAINMENT', label: 'Lazer' },
  { value: 'SHOPPING', label: 'Compras' },
  { value: 'INVESTMENT', label: 'Investimento' },
  { value: 'SALARY', label: 'Salário/Renda' },
  { value: 'HOUSING', label: 'Moradia' },
  { value: 'BILLS', label: 'Contas' },
  { value: 'EDUCATION', label: 'Educação' },
  { value: 'PEOPLE', label: 'Pessoas' },
  { value: 'OTHER', label: 'Outros' },
]

export const PAYMENT_METHODS = [
  { value: 'PIX', label: 'Pix' },
  { value: 'CREDIT_CARD', label: 'Cartão de Crédito' },
  { value: 'DEBIT_CARD', label: 'Cartão de Débito' },
  { value: 'CASH', label: 'Dinheiro' },
  { value: 'OTHER', label: 'Outro' },
]

export const cleanLabel = (label: string) => label.replace(/\s*\(\s*\d+\s*\/\s*\d+\s*\)\s*$/, '').trim()
