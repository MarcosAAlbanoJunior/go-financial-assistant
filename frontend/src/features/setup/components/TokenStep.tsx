import { useMutation } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { sendToken } from '../api'
import { setupErrorMessage } from '../lib/setup'

/** Passo 1: provar que é dono do servidor colando o SETUP_TOKEN do .env (nunca vai na URL). */
export function TokenStep({ onDone }: { onDone: () => void }) {
  const [token, setToken] = useState('')
  const send = useMutation({ mutationFn: sendToken, onSuccess: onDone })

  function submit(e: FormEvent) {
    e.preventDefault()
    send.mutate(token.trim())
  }

  const error = setupErrorMessage(send.error)
  return (
    <form className="setup-step" onSubmit={submit}>
      <h2 className="chart-title">Primeiro, mostre que o servidor é seu</h2>
      <p className="setup-lead">
        O setup só abre com o token que está no arquivo <code>.env</code> do servidor. Assim, ninguém que encontre esta página antes de você
        consegue virar dono do app.
      </p>
      <ol className="setup-howto">
        <li>
          No computador onde o app roda, abra o arquivo <code>.env</code> na pasta do projeto.
        </li>
        <li>
          Copie o valor da linha <code>SETUP_TOKEN=</code> (só o que vem depois do <code>=</code>).
        </li>
      </ol>
      <label className="setup-field">
        Token de setup
        <input
          type="password"
          autoComplete="off"
          spellCheck={false}
          autoFocus
          required
          value={token}
          onChange={(e) => setToken(e.target.value)}
        />
      </label>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <button type="submit" className="btn btn-primary" disabled={send.isPending || !token.trim()}>
        {send.isPending ? 'Conferindo…' : 'Continuar'}
      </button>
    </form>
  )
}
