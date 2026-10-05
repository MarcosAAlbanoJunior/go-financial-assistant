import './styles.css'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { OwnTransfersDialog } from './components/OwnTransfersDialog'
import { QueryState } from '../../shared/components/QueryState'
import { SettingInput } from './components/SettingInput'
import { ConfirmDialog } from './components/ConfirmDialog'
import { SettingsAudit } from './components/SettingsAudit'
import { TEST_TARGET, changedValues, currentValue, needsConfirmation } from './lib/settings'
import { applyOwnTransfers, requestConfirmCode, resetSetting, restartApp, saveSettings, testConnection, useSettings } from './api'
import { ApiError } from '../../shared/api/request'
import type { OwnTransfers, Proof, SettingGroup } from './api'

const errorMessage = (e: unknown) => (e instanceof ApiError ? e.message : 'Não foi possível concluir. Tente de novo.')

export default function Settings() {
  const query = useSettings()
  const queryClient = useQueryClient()
  const [drafts, setDrafts] = useState<Record<string, string>>({})
  const [notice, setNotice] = useState<{ group: string; ok: boolean; text: string } | null>(null)
  const [own, setOwn] = useState<OwnTransfers | null>(null)
  const [restarting, setRestarting] = useState(false)
  // Ação sensível à espera da confirmação (código no chat ou senha).
  const [pending, setPending] = useState<
    { kind: 'save'; group: SettingGroup; values: Record<string, string> } | { kind: 'reset'; key: string; label: string } | null
  >(null)
  const [pwError, setPwError] = useState<string | null>(null)
  const secondFactor = query.data?.secondFactor ?? false

  const save = useMutation({
    mutationFn: ({ values, proof }: { values: Record<string, string>; proof?: Proof }) => saveSettings(values, proof),
    onSuccess: (res, { values }) => {
      setDrafts((d) => Object.fromEntries(Object.entries(d).filter(([k]) => !(k in values))))
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      queryClient.invalidateQueries({ queryKey: ['settings-audit'] })
      if (res.ownTransfers) setOwn(res.ownTransfers)
    },
  })
  const reset = useMutation({
    mutationFn: ({ key, proof }: { key: string; proof?: Proof }) => resetSetting(key, proof),
    onSuccess: (_, { key }) => {
      setDrafts((d) => Object.fromEntries(Object.entries(d).filter(([k]) => k !== key)))
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      queryClient.invalidateQueries({ queryKey: ['settings-audit'] })
    },
  })
  const apply = useMutation({
    mutationFn: applyOwnTransfers,
    onSuccess: () => {
      setOwn(null)
      queryClient.invalidateQueries() // gastos, renda e projeção mudam
    },
  })
  const test = useMutation({ mutationFn: testConnection })

  async function onSave(group: SettingGroup) {
    setNotice(null)
    const values = changedValues(group, drafts)
    if (Object.keys(values).length === 0) return
    if (needsConfirmation(group, Object.keys(values))) {
      openConfirm({ kind: 'save', group, values })
      return
    }
    await doSave(group, values)
  }

  async function doSave(group: SettingGroup, values: Record<string, string>, proof?: Proof) {
    try {
      await save.mutateAsync({ values, proof })
      setPending(null)
      setNotice({ group: group.id, ok: true, text: 'Salvo.' })
    } catch (e) {
      if (proof) setPwError(errorMessage(e))
      else setNotice({ group: group.id, ok: false, text: errorMessage(e) })
    }
  }

  function onReset(group: SettingGroup, key: string) {
    const field = group.fields.find((f) => f.key === key)
    if (field?.sensitive) {
      openConfirm({ kind: 'reset', key, label: field.label })
      return
    }
    reset.mutate({ key })
  }

  // Com o segundo fator, abrir a confirmação já pede o código ao chat.
  function openConfirm(p: NonNullable<typeof pending>) {
    setPwError(null)
    setPending(p)
    if (secondFactor) sendCode()
  }

  async function sendCode() {
    try {
      await requestConfirmCode()
      setPwError(null)
    } catch (e) {
      setPwError(errorMessage(e))
    }
  }

  async function confirmPending(secret: string) {
    if (!pending) return
    const proof: Proof = secondFactor ? { code: secret } : { password: secret }
    if (pending.kind === 'save') {
      await doSave(pending.group, pending.values, proof)
      return
    }
    try {
      await reset.mutateAsync({ key: pending.key, proof })
      setPending(null)
    } catch (e) {
      setPwError(errorMessage(e))
    }
  }

  async function onTest(group: SettingGroup) {
    const target = TEST_TARGET[group.id]
    if (!target) return
    setNotice(null)
    try {
      const res = await test.mutateAsync(target)
      setNotice({ group: group.id, ok: res.ok, text: res.message })
    } catch (e) {
      setNotice({ group: group.id, ok: false, text: errorMessage(e) })
    }
  }

  async function onRestart() {
    setRestarting(true)
    try {
      await restartApp()
    } catch {
      setRestarting(false)
      return
    }
    // O Docker sobe o app de novo em segundos: espera responder (ou desiste depois de ~1 min).
    for (let i = 0; i < 30; i++) {
      await new Promise((r) => setTimeout(r, 2000))
      try {
        const res = await fetch('/api/me', { credentials: 'same-origin' })
        if (res.ok) break
      } catch {
        // ainda reiniciando
      }
    }
    setRestarting(false)
    queryClient.invalidateQueries()
  }

  return (
    <>
      <h1 className="page-title">Configurações</h1>
      <p className="tile-note set-intro">
        O que você salva aqui vale mais que o arquivo .env; o que não for salvo continua vindo dele. Chaves de acesso, senha do dashboard, banco de dados e canal só mudam no ambiente.
      </p>

      <QueryState query={query}>
        {(data) => (
          <>
            {data.restartPending && (
              <div className="notice set-restart" role="status">
                <span>Algumas configurações só valem depois de reiniciar o app.</span>
                <button type="button" className="btn btn-primary" disabled={restarting} onClick={onRestart}>
                  {restarting ? 'Reiniciando…' : 'Reiniciar agora'}
                </button>
              </div>
            )}
            {!data.secondFactor && (
              <p className="notice" role="note">
                Segundo fator desligado: o login e as alterações sensíveis pedem só a senha. Ele liga sozinho quando o canal (Telegram ou WhatsApp) está no ar e{' '}
                <code>DASHBOARD_2FA</code> não é <code>off</code>.
              </p>
            )}
            {!data.encryption && (
              <p className="notice" role="note">
                Para guardar chaves e tokens aqui, defina <code>APP_SECRET_KEY</code> (mínimo 16 caracteres) no ambiente e reinicie. Ela cifra os segredos no banco e nunca é salva nele.
              </p>
            )}

            {data.groups.map((group) => {
              const dirty = Object.keys(changedValues(group, drafts)).length > 0
              const target = TEST_TARGET[group.id]
              return (
                <section key={group.id} className="card set-card" aria-labelledby={`g-${group.id}`}>
                  <h2 className="chart-title" id={`g-${group.id}`}>
                    {group.title}
                  </h2>
                  <p className="tile-note">{group.help}</p>
                  <div className="set-fields">
                    {group.fields.map((f) => (
                      <SettingInput
                        key={f.key}
                        field={f}
                        value={currentValue(f, drafts)}
                        encryption={data.encryption}
                        busy={save.isPending || reset.isPending}
                        onChange={(v) => setDrafts((d) => ({ ...d, [f.key]: v }))}
                        onReset={() => onReset(group, f.key)}
                      />
                    ))}
                  </div>
                  <div className="form-actions">
                    <button type="button" className="btn btn-primary" disabled={!dirty || save.isPending} onClick={() => onSave(group)}>
                      {save.isPending ? 'Salvando…' : 'Salvar'}
                    </button>
                    {target && (
                      <button type="button" className="btn" disabled={test.isPending || dirty} title={dirty ? 'Salve antes de testar' : undefined} onClick={() => onTest(group)}>
                        {test.isPending ? 'Testando…' : 'Testar conexão'}
                      </button>
                    )}
                  </div>
                  {notice?.group === group.id && (
                    <p className={notice.ok ? 'set-ok' : 'error'} role={notice.ok ? 'status' : 'alert'}>
                      {notice.text}
                    </p>
                  )}
                </section>
              )
            })}
          </>
        )}
      </QueryState>

      <SettingsAudit />

      {pending && (
        <ConfirmDialog
          mode={secondFactor ? 'code' : 'password'}
          onResend={sendCode}
          action={pending.kind === 'save' ? `Salvar ${Object.keys(pending.values).map((k) => pending.group.fields.find((f) => f.key === k)?.label ?? k).join(', ')}` : `Restaurar ${pending.label} ao valor do ambiente`}
          busy={save.isPending || reset.isPending}
          error={pwError}
          onConfirm={confirmPending}
          onCancel={() => setPending(null)}
        />
      )}

      {own && (
        <OwnTransfersDialog
          data={own}
          busy={apply.isPending}
          onApply={() => apply.mutate()}
          onKeep={() => setOwn(null)}
        />
      )}
      {apply.isError && (
        <p className="error" role="alert">
          {errorMessage(apply.error)}
        </p>
      )}
    </>
  )
}
