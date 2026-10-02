import { useEffect, useRef, useState } from 'react'

interface Props {
  /** O que será feito, em uma frase (ex.: "Salvar Token do bot"). */
  action: string
  busy: boolean
  error: string | null
  onConfirm: (password: string) => void
  onCancel: () => void
}

/** Confirma a senha do dashboard antes de mexer em algo sensível: uma sessão roubada não basta para trocar chaves. */
export function PasswordDialog({ action, busy, error, onConfirm, onCancel }: Props) {
  const ref = useRef<HTMLDialogElement>(null)
  const [password, setPassword] = useState('')
  useEffect(() => {
    const d = ref.current
    if (d && !d.open) d.showModal()
  }, [])

  return (
    <dialog ref={ref} className="set-dialog" aria-labelledby="pw-title" onCancel={(e) => (busy ? e.preventDefault() : onCancel())}>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          if (password) onConfirm(password)
        }}
      >
        <h2 id="pw-title" className="chart-title">
          Confirme a senha
        </h2>
        <p>{action}. Por segurança, digite de novo a senha do dashboard.</p>
        <label className="set-field">
          <span className="set-label">Senha do dashboard</span>
          <input type="password" autoComplete="current-password" autoFocus value={password} disabled={busy} onChange={(e) => setPassword(e.target.value)} />
        </label>
        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}
        <div className="form-actions">
          <button type="submit" className="btn btn-primary" disabled={busy || !password}>
            {busy ? 'Confirmando…' : 'Confirmar'}
          </button>
          <button type="button" className="btn" disabled={busy} onClick={onCancel}>
            Cancelar
          </button>
        </div>
      </form>
    </dialog>
  )
}
