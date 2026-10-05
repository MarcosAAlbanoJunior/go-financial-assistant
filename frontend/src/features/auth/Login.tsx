import './styles.css'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { login, loginCode } from './api'
import { loginErrorMessage } from './lib/login'

export default function Login() {
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  // Com o segundo fator, a senha certa leva ao passo do código (enviado ao chat).
  const [step, setStep] = useState<'password' | 'code'>('password')
  const [resent, setResent] = useState(false)
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  async function enter() {
    // reset (e não invalidate): um 401 anterior fica em cache e mandaria de volta ao login antes de revalidar.
    await queryClient.resetQueries({ queryKey: ['me'] })
    navigate('/', { replace: true })
  }

  const passwordStep = useMutation({
    mutationFn: login,
    onSuccess: (res) => {
      if (res.step !== 'code') return enter()
      setCode('')
      setStep('code')
    },
  })
  const codeStep = useMutation({ mutationFn: loginCode, onSuccess: enter })

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (step === 'password') passwordStep.mutate(password)
    else codeStep.mutate(code)
  }

  // Reenviar é pedir o código de novo com a mesma senha (o servidor exige um minuto entre envios).
  function resend() {
    codeStep.reset()
    setResent(false)
    passwordStep.mutate(password, { onSuccess: () => setResent(true) })
  }

  function back() {
    passwordStep.reset()
    codeStep.reset()
    setStep('password')
    setResent(false)
  }

  const message = loginErrorMessage(step === 'password' ? passwordStep.error : (codeStep.error ?? passwordStep.error), step)
  const busy = passwordStep.isPending || codeStep.isPending

  return (
    <div className="login">
      <form className="card" onSubmit={handleSubmit}>
        <h1 className="chart-title">FinAssist</h1>
        {step === 'password' ? (
          <label>
            Senha
            <input
              type="password"
              autoComplete="current-password"
              autoFocus
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </label>
        ) : (
          <>
            <p className="login-hint" role="status">
              {resent ? 'Enviamos um código novo ao seu chat.' : 'Enviamos um código ao seu chat. Ele vale 5 minutos.'}
            </p>
            <label>
              Código
              <input
                inputMode="numeric"
                autoComplete="one-time-code"
                pattern="[0-9]{8}"
                maxLength={8}
                autoFocus
                required
                value={code}
                onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
              />
            </label>
          </>
        )}
        {message && (
          <p className="error" role="alert">
            {message}
          </p>
        )}
        <button type="submit" className="btn btn-primary" disabled={busy}>
          {busy ? 'Entrando…' : step === 'password' ? 'Continuar' : 'Entrar'}
        </button>
        {step === 'code' && (
          <div className="login-links">
            <button type="button" className="btn" disabled={busy} onClick={resend}>
              Reenviar código
            </button>
            <button type="button" className="btn" disabled={busy} onClick={back}>
              Voltar
            </button>
          </div>
        )}
      </form>
    </div>
  )
}
