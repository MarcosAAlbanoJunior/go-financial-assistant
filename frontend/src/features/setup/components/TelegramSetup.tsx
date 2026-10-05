import { useMutation, useQuery } from '@tanstack/react-query'
import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { ApiError } from '../../../shared/api/request'
import { acceptCandidate, confirmCode, pollStart, rejectCandidate, saveBot, type SetupStatus } from '../api'
import { POLL_EVERY_MS, START_WAIT_MS, setupErrorMessage, telegramStage } from '../lib/setup'

interface Props {
  status: SetupStatus
  onChanged: () => void
  onFinished: (bot: string) => void
}

/** Telegram em quatro etapas: criar o bot, mandar /start (sem precisar descobrir o ID), confirmar quem é e digitar o código. */
export function TelegramSetup({ status, onChanged, onFinished }: Props) {
  const [changingBot, setChangingBot] = useState(false)
  const stage = changingBot ? 'bot' : telegramStage(status.telegram)
  const bot = status.telegram?.bot ?? ''
  // Sessão de setup vencida (401) em qualquer etapa: o estado recarregado leva de volta ao token.
  const onError = useCallback(
    (e: Error) => {
      if (e instanceof ApiError && e.status === 401 && e.message.startsWith('sessão de setup')) onChanged()
    },
    [onChanged],
  )

  if (stage === 'bot') {
    return (
      <BotForm
        current={bot}
        onCancel={changingBot ? () => setChangingBot(false) : undefined}
        onDone={() => {
          setChangingBot(false)
          onChanged()
        }}
        onError={onError}
      />
    )
  }
  if (stage === 'start') return <WaitStart bot={bot} onFound={onChanged} onChangeBot={() => setChangingBot(true)} onError={onError} />
  if (stage === 'confirm-person') return <ConfirmPerson status={status} onChanged={onChanged} onError={onError} />
  return <CodeForm bot={bot} onChanged={onChanged} onFinished={onFinished} onError={onError} />
}

function BotForm({ current, onCancel, onDone, onError }: { current: string; onCancel?: () => void; onDone: () => void; onError: (e: Error) => void }) {
  const [token, setToken] = useState('')
  const save = useMutation({ mutationFn: saveBot, onSuccess: onDone, onError })

  function submit(e: FormEvent) {
    e.preventDefault()
    save.mutate(token.trim())
  }

  const error = setupErrorMessage(save.error)
  return (
    <form className="setup-substep" onSubmit={submit}>
      <h3 className="setup-subtitle">1. Crie o bot</h3>
      <ol className="setup-howto">
        <li>
          No Telegram, abra o{' '}
          <a href="https://t.me/BotFather" target="_blank" rel="noreferrer">
            @BotFather
          </a>{' '}
          (o bot oficial que cria bots).
        </li>
        <li>
          Mande <code>/newbot</code>.
        </li>
        <li>Escolha um nome (ex.: Minhas Finanças) e um usuário terminado em "bot" (ex.: minhas_financas_bot).</li>
        <li>
          Ele responde com um token parecido com <code>123456789:ABCdef…</code>. Copie e cole aqui.
        </li>
      </ol>
      <label className="setup-field">
        Token do bot
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
      <div className="setup-actions">
        <button type="submit" className="btn btn-primary" disabled={save.isPending || !token.trim()}>
          {save.isPending ? 'Testando o token…' : 'Testar e continuar'}
        </button>
        {onCancel && (
          <button type="button" className="btn" onClick={onCancel}>
            Manter @{current}
          </button>
        )}
      </div>
    </form>
  )
}

function WaitStart({ bot, onFound, onChangeBot, onError }: { bot: string; onFound: () => void; onChangeBot: () => void; onError: (e: Error) => void }) {
  const [round, setRound] = useState(0)
  const [timedOut, setTimedOut] = useState(false)
  const poll = useQuery({
    queryKey: ['setup-poll', bot, round],
    queryFn: pollStart,
    enabled: !timedOut,
    refetchInterval: POLL_EVERY_MS,
    retry: false,
    gcTime: 0,
  })
  const found = !!poll.data?.candidate

  useEffect(() => {
    if (found) onFound()
  }, [found, onFound])
  useEffect(() => {
    if (poll.error) onError(poll.error)
  }, [poll.error, onError])
  // Espera até 5 minutos; depois, a pessoa escolhe continuar esperando.
  useEffect(() => {
    const id = window.setTimeout(() => setTimedOut(true), START_WAIT_MS)
    return () => window.clearTimeout(id)
  }, [round])

  const error = setupErrorMessage(poll.error)
  return (
    <div className="setup-substep">
      <p className="setup-done-line">
        <span aria-hidden="true">✓</span> Bot @{bot} encontrado.{' '}
        <button type="button" className="setup-link" onClick={onChangeBot}>
          trocar
        </button>
      </p>
      <h3 className="setup-subtitle">2. Mande /start para o seu bot</h3>
      <p className="setup-lead">Assim o app descobre a sua conta sozinho, sem você precisar procurar o seu ID.</p>
      <ol className="setup-howto">
        <li>Abra o seu bot no Telegram pelo botão abaixo (no celular ou no computador).</li>
        <li>
          Toque em <strong>Iniciar</strong> (ou mande <code>/start</code>).
        </li>
      </ol>
      <a className="btn btn-primary setup-cta" href={`https://t.me/${bot}`} target="_blank" rel="noreferrer">
        Abrir t.me/{bot}
      </a>
      {timedOut ? (
        <div className="setup-wait">
          <p>Não chegou nenhuma mensagem em 5 minutos.</p>
          <button
            type="button"
            className="btn"
            onClick={() => {
              setTimedOut(false)
              setRound((r) => r + 1)
            }}
          >
            Continuar esperando
          </button>
        </div>
      ) : (
        <p className="setup-wait" role="status">
          <span className="setup-spinner" aria-hidden="true" /> Esperando a sua mensagem…
        </p>
      )}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
    </div>
  )
}

function ConfirmPerson({ status, onChanged, onError }: { status: SetupStatus; onChanged: () => void; onError: (e: Error) => void }) {
  const accept = useMutation({ mutationFn: acceptCandidate, onSuccess: onChanged, onError })
  const reject = useMutation({ mutationFn: rejectCandidate, onSuccess: onChanged, onError })
  const c = status.telegram?.candidate
  const error = setupErrorMessage(accept.error ?? reject.error)
  return (
    <div className="setup-substep">
      <h3 className="setup-subtitle">É você?</h3>
      <p className="setup-lead">
        Recebemos a mensagem de <strong>{c?.name || 'alguém sem nome'}</strong>
        {c?.username ? ` (@${c.username})` : ''}. É você?
      </p>
      <p className="tile-note">Só essa conta vai poder conversar com o bot. Se não for você, alguém achou o bot antes: diga que não e mande /start de novo.</p>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <div className="setup-actions">
        <button type="button" className="btn btn-primary" disabled={accept.isPending || reject.isPending} onClick={() => accept.mutate()}>
          {accept.isPending ? 'Enviando o código…' : 'Sim, sou eu'}
        </button>
        <button type="button" className="btn" disabled={accept.isPending || reject.isPending} onClick={() => reject.mutate()}>
          Não, não sou eu
        </button>
      </div>
    </div>
  )
}

function CodeForm({ bot, onChanged, onFinished, onError }: { bot: string; onChanged: () => void; onFinished: (bot: string) => void; onError: (e: Error) => void }) {
  const [code, setCode] = useState('')
  const [resent, setResent] = useState(false)
  const confirm = useMutation({ mutationFn: confirmCode, onSuccess: () => onFinished(bot), onError })
  const resend = useMutation({ mutationFn: acceptCandidate, onSuccess: () => setResent(true), onError })
  const reject = useMutation({ mutationFn: rejectCandidate, onSuccess: onChanged, onError })

  function submit(e: FormEvent) {
    e.preventDefault()
    confirm.mutate(code)
  }

  const busy = confirm.isPending || resend.isPending || reject.isPending
  const error = setupErrorMessage(confirm.error ?? resend.error ?? reject.error)
  return (
    <form className="setup-substep" onSubmit={submit}>
      <h3 className="setup-subtitle">3. Digite o código</h3>
      <p className="setup-lead" role="status">
        {resent ? 'Mandamos um código novo' : 'Mandamos um código de 6 dígitos'} para você no Telegram, pelo @{bot}. Ele vale 5 minutos.
      </p>
      <label className="setup-field">
        Código
        <input
          inputMode="numeric"
          autoComplete="one-time-code"
          pattern="[0-9]{6}"
          maxLength={6}
          autoFocus
          required
          value={code}
          onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
        />
      </label>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <div className="setup-actions">
        <button type="submit" className="btn btn-primary" disabled={busy || code.length !== 6}>
          {confirm.isPending ? 'Concluindo…' : 'Confirmar e concluir'}
        </button>
        <button type="button" className="btn" disabled={busy} onClick={() => resend.mutate()}>
          Mandar outro código
        </button>
        <button type="button" className="btn" disabled={busy} onClick={() => reject.mutate()}>
          Não sou eu
        </button>
      </div>
    </form>
  )
}
