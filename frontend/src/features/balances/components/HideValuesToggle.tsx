import { Eye, EyeOff } from 'lucide-react'

export function HideValuesToggle({ hidden, onToggle }: { hidden: boolean; onToggle: () => void }) {
  const Icon = hidden ? EyeOff : Eye
  return (
    <button type="button" className="btn bal-btn" aria-pressed={hidden} onClick={onToggle}>
      <Icon size={16} aria-hidden="true" /> {hidden ? 'Mostrar valores' : 'Ocultar valores'}
    </button>
  )
}
