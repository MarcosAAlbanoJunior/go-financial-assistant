import { useEffect, useRef, useState } from 'react'

interface Props {
  /** O que será feito, em uma frase (ex.: "Salvar Token do bot"). */
  action: string
  /** Com o segundo fator, o código enviado ao chat; sem ele, a senha do dashboard de novo. */
  mode: 'password' | 'code'
  busy: boolean
  error: string | null
  onConfirm: (secret: string) => void
  onResend: () => void
  onCancel: () => void
}

/** Confirma algo sensível antes de fazer: uma sessão roubada não basta para trocar chaves. */
export function ConfirmDialog({ action, mode, busy, error, onConfirm, onResend, onCancel }: Props) {
  const ref = useRef<HTMLDialogElement>(null)
  const [secret, setSecret] = useState('')
  useEffect(() => {
    const d = ref.current
    if (d && !d.open) d.showModal()
  }, [])
  const code = mode === 'code'

  return (
    <dialog ref={ref} className="set-dialog" aria-labelledby="confirm-title" onCancel={(e) => (busy ? e.preventDefault() : onCancel())}>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          if (secret) onConfirm(secret)
        }}
      >
        <h2 id="confirm-title" className="chart-title">
          {code ? 'Confirme com o código' : 'Confirme a senha'}
        </h2>
        <p>
          {action}.{' '}
          {code ? 'Por segurança, enviamos um código ao seu chat. Ele vale 5 minutos e serve só para esta alteração.' : 'Por segurança, digite de novo a senha do dashboard.'}
        </p>
        <label className="set-field">
          <span className="set-label">{code ? 'Código' : 'Senha do dashboard'}</span>
          {code ? (
            <input
              inputMode="numeric"
              autoComplete="one-time-code"
              maxLength={8}
              autoFocus
              value={secret}
              disabled={busy}
              onChange={(e) => setSecret(e.target.value.replace(/\D/g, ''))}
            />
          ) : (
            <input type="password" autoComplete="current-password" autoFocus value={secret} disabled={busy} onChange={(e) => setSecret(e.target.value)} />
          )}
        </label>
        {error && (
          <p className="error" role="alert">
            {error}
          </p>
        )}
        <div className="form-actions">
          <button type="submit" className="btn btn-primary" disabled={busy || !secret}>
            {busy ? 'Confirmando…' : 'Confirmar'}
          </button>
          {code && (
            <button type="button" className="btn" disabled={busy} onClick={onResend}>
              Reenviar código
            </button>
          )}
          <button type="button" className="btn" disabled={busy} onClick={onCancel}>
            Cancelar
          </button>
        </div>
      </form>
    </dialog>
  )
}
