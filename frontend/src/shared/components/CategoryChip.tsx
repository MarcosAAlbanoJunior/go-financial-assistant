import type { CSSProperties } from 'react'
import { categoryVisual } from '../lib/categoryVisual'

/** Ícone da categoria sobre um fundo suave da sua cor. Decorativo: o nome vem sempre ao lado. */
export function CategoryChip({ category, size = 18 }: { category: string; size?: number }) {
  const { color, Icon } = categoryVisual(category)
  return (
    <span className="chip" style={{ '--cat': color } as CSSProperties}>
      <Icon size={size} aria-hidden="true" />
    </span>
  )
}
