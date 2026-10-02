import './styles.css'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { login } from './api'
import { ApiError } from '../../shared/api/request'

export default function Login() {
  const [password, setPassword] = useState('')
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  const mutation = useMutation({
    mutationFn: login,
    onSuccess: async () => {
      // reset (e não invalidate): um 401 anterior fica em cache e mandaria de volta ao login antes de revalidar.
      await queryClient.resetQueries({ queryKey: ['me'] })
      navigate('/', { replace: true })
    },
  })

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    mutation.mutate(password)
  }

  const error = mutation.error
  const message =
    error instanceof ApiError && error.status === 429
      ? 'Muitas tentativas. Aguarde um minuto e tente de novo.'
      : error instanceof ApiError && error.status === 401
        ? 'Senha incorreta.'
        : error
          ? 'Não foi possível entrar. Tente de novo.'
          : null

  return (
    <div className="login">
      <form className="card" onSubmit={handleSubmit}>
        <h1 className="chart-title">FinAssist</h1>
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
        {message && (
          <p className="error" role="alert">
            {message}
          </p>
        )}
        <button type="submit" className="btn btn-primary" disabled={mutation.isPending}>
          {mutation.isPending ? 'Entrando…' : 'Entrar'}
        </button>
      </form>
    </div>
  )
}
