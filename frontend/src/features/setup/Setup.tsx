import './styles.css'
import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useState, type ReactNode } from 'react'
import { Navigate } from 'react-router'
import { ApiError } from '../../shared/api/request'
import { useSetupStatus } from './api'
import { Blocked } from './components/Blocked'
import { ChannelStep } from './components/ChannelStep'
import { DoneStep } from './components/DoneStep'
import { PasswordStep } from './components/PasswordStep'
import { Progress } from './components/Progress'
import { TokenStep } from './components/TokenStep'
import { currentStep } from './lib/setup'

/** Setup pelo navegador: do zero até o dashboard, liberado pelo SETUP_TOKEN do .env. O estado fica no servidor. */
export default function Setup() {
  const queryClient = useQueryClient()
  // Ao concluir, a tela "Pronto" fica mesmo com /api/setup/status passando a responder 404.
  const [finished, setFinished] = useState<{ reopen: boolean; bot: string } | null>(null)
  const [editingPassword, setEditingPassword] = useState(false)
  const status = useSetupStatus(!finished)
  const refresh = useCallback(() => void queryClient.invalidateQueries({ queryKey: ['setup-status'] }), [queryClient])

  if (finished) {
    return (
      <Shell step="done" reopen={finished.reopen}>
        <DoneStep reopen={finished.reopen} bot={finished.bot} />
      </Shell>
    )
  }
  if (status.isPending) return <p className="state">Carregando…</p>
  if (status.error instanceof ApiError && status.error.status === 404) return <Navigate to="/login" replace />
  if (status.isError) {
    return (
      <Shell>
        <p className="error" role="alert">
          Não foi possível falar com o app. Confira se ele está no ar (docker compose ps) e recarregue a página.
        </p>
      </Shell>
    )
  }

  const s = status.data
  if (s.problemKind) {
    return (
      <Shell reopen={s.reopen}>
        <Blocked kind={s.problemKind} message={s.problem ?? ''} minLength={s.tokenMinLength} />
      </Shell>
    )
  }

  const step = currentStep(s, editingPassword)
  return (
    <Shell step={step} reopen={s.reopen}>
      {step === 'token' && <TokenStep onDone={refresh} />}
      {step === 'password' && (
        <PasswordStep
          status={s}
          onDone={() => {
            setEditingPassword(false)
            refresh()
          }}
          onCancel={editingPassword ? () => setEditingPassword(false) : undefined}
          onSessionExpired={refresh}
        />
      )}
      {step === 'channel' && (
        <ChannelStep
          status={s}
          onEditPassword={() => setEditingPassword(true)}
          onChanged={refresh}
          onFinished={(bot) => {
            // A sessão do dashboard já veio na resposta: /api/me precisa ser lido de novo.
            void queryClient.resetQueries({ queryKey: ['me'] })
            setFinished({ reopen: s.reopen, bot })
          }}
        />
      )}
    </Shell>
  )
}

function Shell({ step, reopen, children }: { step?: Parameters<typeof Progress>[0]['step']; reopen?: boolean; children: ReactNode }) {
  return (
    <main className="setup">
      <div className="setup-box">
        <header className="setup-head">
          <h1 className="page-title">Configurar o FinAssist</h1>
          {step && <Progress step={step} />}
        </header>
        {reopen && step !== 'done' && (
          <p className="setup-note" role="status">
            Setup reaberto (SETUP_REOPEN): redefina a senha e/ou o canal. Seus dados continuam como estão.
          </p>
        )}
        <section className="card setup-card">{children}</section>
      </div>
    </main>
  )
}
