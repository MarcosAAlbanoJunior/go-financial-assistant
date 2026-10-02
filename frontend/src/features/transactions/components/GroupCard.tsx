import { ChevronDown } from 'lucide-react'
import { useId, useState, type CSSProperties, type ReactNode } from 'react'

interface Props {
  icon: ReactNode
  title: string
  subtitle: string
  /** Valor principal do cabeçalho (ex.: despesas do grupo). */
  amount: string
  /** Linhas menores abaixo do valor (ex.: receitas, investimentos). */
  extras?: string[]
  /** Participação no maior grupo, de 0 a 1, desenhada como barra. */
  share: number
  color: string
  /** O detalhe só é montado (e buscado) quando o card é aberto. */
  children: () => ReactNode
}

export function GroupCard({ icon, title, subtitle, amount, extras = [], share, color, children }: Props) {
  const [open, setOpen] = useState(false)
  const panelId = useId()

  return (
    <article className={`group-card ${open ? 'is-open' : ''}`} style={{ '--cat': color } as CSSProperties}>
      <button type="button" className="group-head" aria-expanded={open} aria-controls={panelId} onClick={() => setOpen(!open)}>
        {icon}
        <span className="group-title">
          <strong>{title}</strong>
          <span className="group-sub">{subtitle}</span>
        </span>
        <span className="group-amount">
          <strong>{amount}</strong>
          {extras.map((e) => (
            <span key={e} className="group-sub">
              {e}
            </span>
          ))}
        </span>
        <ChevronDown size={18} className="group-chevron" aria-hidden="true" />
      </button>
      <div className="group-bar" aria-hidden="true">
        <div style={{ width: `${Math.max(2, share * 100)}%` }} />
      </div>
      {open && (
        <div id={panelId} className="group-body">
          {children()}
        </div>
      )}
    </article>
  )
}
