import { useMutation } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { ApiError } from '../../../shared/api/request'
import { savePassword, type SetupStatus } from '../api'
import { passwordStrength, setupErrorMessage } from '../lib/setup'

interface Props {
  status: SetupStatus
  onDone: () => void
  onCancel?: () => void
  onSessionExpired: () => void
}

/** Passo 2: a senha do dashboard. Só passa a valer para entrar quando o setup terminar. */
export function PasswordStep({ status, onDone, onCancel, onSessionExpired }: Props) {
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const save = useMutation({
    mutationFn: savePassword,
    onSuccess: onDone,
    onError: (e) => {
      if (e instanceof ApiError && e.status === 401) onSessionExpired()
    },
  })
  const min = status.passwordMinLength
  const strength = passwordStrength(password, min)
  const mismatch = confirm !== '' && confirm !== password

  function submit(e: FormEvent) {
    e.preventDefault()
    if (strength.score > 0 && !mismatch) save.mutate(password)
  }

  const fromEnv = status.password?.fromEnv && !status.password.draft
  const error = setupErrorMessage(save.error)
  return (
    <form className="setup-step" onSubmit={submit}>
      <h2 className="chart-title">Senha do dashboard</h2>
      <p className="setup-lead">
        Junto com ela, o login vai pedir um código no seu chat. Não precisa ser enorme, mas não use uma senha de outro site.
      </p>
      {fromEnv && (
        <p className="setup-note">
          Já há uma senha no .env (<code>DASHBOARD_PASSWORD</code>). Uma senha definida aqui passa a valer no lugar dela.
        </p>
      )}
      <label className="setup-field">
        Senha (mínimo de {min} caracteres)
        <input type="password" autoComplete="new-password" autoFocus required value={password} onChange={(e) => setPassword(e.target.value)} />
      </label>
      <div className="setup-meter" aria-hidden="true">
        {[1, 2, 3].map((i) => (
          <span key={i} className={strength.score >= i ? `is-on score-${strength.score}` : ''} />
        ))}
      </div>
      <p className="tile-note" role="status">
        {password ? strength.label : `Use ao menos ${min} caracteres. Uma frase com espaços é ótima.`}
      </p>
      <label className="setup-field">
        Repita a senha
        <input type="password" autoComplete="new-password" required value={confirm} onChange={(e) => setConfirm(e.target.value)} />
      </label>
      {mismatch && (
        <p className="error" role="alert">
          As senhas não são iguais.
        </p>
      )}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <div className="setup-actions">
        <button type="submit" className="btn btn-primary" disabled={save.isPending || strength.score === 0 || confirm !== password}>
          {save.isPending ? 'Salvando…' : 'Salvar e continuar'}
        </button>
        {onCancel && (
          <button type="button" className="btn" onClick={onCancel}>
            Cancelar
          </button>
        )}
      </div>
    </form>
  )
}
