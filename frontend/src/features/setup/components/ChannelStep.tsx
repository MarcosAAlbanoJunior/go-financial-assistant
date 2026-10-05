import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { ApiError } from '../../../shared/api/request'
import { keepChannel, type SetupStatus } from '../api'
import { setupErrorMessage } from '../lib/setup'
import { TelegramSetup } from './TelegramSetup'

interface Props {
  status: SetupStatus
  onEditPassword: () => void
  onChanged: () => void
  onFinished: (bot: string) => void
}

/** Passo 3: o canal de conversa (Telegram). Um canal que já está no .env pode ser mantido. */
export function ChannelStep({ status, onEditPassword, onChanged, onFinished }: Props) {
  const channel = status.channel ?? { configured: false, canReplace: true }
  // Reaberto com canal: a pessoa escolhe manter o de antes ou configurar outro bot.
  const [replace, setReplace] = useState(!channel.configured || !!status.telegram?.bot)
  const keep = useMutation({
    mutationFn: keepChannel,
    onSuccess: () => onFinished(''),
    onError: (e) => {
      if (e instanceof ApiError && e.status === 401) onChanged()
    },
  })

  return (
    <div className="setup-step">
      <PasswordSummary status={status} onEdit={onEditPassword} />

      {channel.configured && !channel.canReplace && (
        <>
          <h2 className="chart-title">Canal de conversa</h2>
          <p className="setup-lead">O canal já está definido no .env e continua como está. Falta só concluir.</p>
          <KeepButton pending={keep.isPending} error={keep.error} onClick={() => keep.mutate()} />
        </>
      )}

      {channel.configured && channel.canReplace && !replace && (
        <>
          <h2 className="chart-title">Canal de conversa</h2>
          <p className="setup-lead">Você pode manter o canal de antes ou ligar outro bot do Telegram (por exemplo, se perdeu o acesso ao chat).</p>
          <KeepButton pending={keep.isPending} error={keep.error} onClick={() => keep.mutate()} />
          <button type="button" className="btn" onClick={() => setReplace(true)}>
            Configurar outro bot do Telegram
          </button>
        </>
      )}

      {channel.canReplace && replace && (
        <>
          <h2 className="chart-title">Canal de conversa: Telegram</h2>
          <TelegramSetup status={status} onChanged={onChanged} onFinished={onFinished} />
          {channel.configured && !status.telegram?.bot && (
            <button type="button" className="btn" onClick={() => setReplace(false)}>
              Voltar: manter o canal de antes
            </button>
          )}
        </>
      )}
    </div>
  )
}

function KeepButton({ pending, error, onClick }: { pending: boolean; error: unknown; onClick: () => void }) {
  const message = setupErrorMessage(error)
  return (
    <>
      {message && (
        <p className="error" role="alert">
          {message}
        </p>
      )}
      <button type="button" className="btn btn-primary" disabled={pending} onClick={onClick}>
        {pending ? 'Concluindo…' : 'Manter o canal e concluir'}
      </button>
    </>
  )
}

function PasswordSummary({ status, onEdit }: { status: SetupStatus; onEdit: () => void }) {
  const p = status.password
  const text = p?.draft ? 'Senha definida' : p?.fromEnv ? 'Senha definida no .env' : 'Senha de antes mantida'
  return (
    <p className="setup-done-line">
      <span aria-hidden="true">✓</span> {text}.{' '}
      <button type="button" className="setup-link" onClick={onEdit}>
        trocar
      </button>
    </p>
  )
}
