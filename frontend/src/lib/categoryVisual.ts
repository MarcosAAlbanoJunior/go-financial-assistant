import {
  Car,
  CircleEllipsis,
  Clapperboard,
  HeartPulse,
  ShoppingBag,
  ShoppingCart,
  TrendingUp,
  Utensils,
  Wallet,
  type LucideIcon,
} from 'lucide-react'

export interface CategoryVisual {
  /** Variável CSS da cor da categoria (definida nos temas, em index.css). */
  color: string
  Icon: LucideIcon
}

// A cor segue a categoria em todas as telas (nunca a posição no ranking). O ícone e o nome
// sempre acompanham a cor, então ela nunca é o único sinal.
const VISUALS: Record<string, CategoryVisual> = {
  FOOD: { color: 'var(--cat-food)', Icon: Utensils },
  MARKET: { color: 'var(--cat-market)', Icon: ShoppingCart },
  TRANSPORT: { color: 'var(--cat-transport)', Icon: Car },
  HEALTH: { color: 'var(--cat-health)', Icon: HeartPulse },
  ENTERTAINMENT: { color: 'var(--cat-entertainment)', Icon: Clapperboard },
  SHOPPING: { color: 'var(--cat-shopping)', Icon: ShoppingBag },
  INVESTMENT: { color: 'var(--cat-investment)', Icon: TrendingUp },
  SALARY: { color: 'var(--cat-salary)', Icon: Wallet },
  OTHER: { color: 'var(--cat-other)', Icon: CircleEllipsis },
}

export const categoryVisual = (category: string): CategoryVisual => VISUALS[category] ?? VISUALS.OTHER
