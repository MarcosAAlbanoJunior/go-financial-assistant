import { SOURCE_LABEL, WEEKDAY_LABEL, fieldStatus } from '../lib/settings'
import type { SettingField } from '../api'

interface Props {
  field: SettingField
  value: string
  /** Segredos só podem ser guardados com APP_SECRET_KEY no ambiente. */
  encryption: boolean
  busy: boolean
  onChange: (value: string) => void
  onReset: () => void
}

/** Um campo da página de configurações, de acordo com o tipo, com a origem do valor e o aviso de reinício. */
export function SettingInput({ field: f, value, encryption, busy, onChange, onReset }: Props) {
  const id = `set-${f.key}`
  const helpId = `${id}-help`
  const disabled = busy || (f.kind === 'secret' && !encryption)

  return (
    <div className="set-field">
      <div className="set-head">
        <label htmlFor={id} className="set-label">
          {f.label}
        </label>
        <span className="set-badges">
          {f.kind === 'secret' && <span className={`badge ${f.isSet ? 'badge-ok' : ''}`}>{fieldStatus(f)}</span>}
          <span className="badge">{SOURCE_LABEL[f.source]}</span>
          {!f.live && <span className="badge">vale após reiniciar</span>}
        </span>
      </div>

      {f.kind === 'bool' ? (
        <label className="check">
          <input id={id} type="checkbox" checked={value === 'true'} disabled={disabled} aria-describedby={helpId} onChange={(e) => onChange(String(e.target.checked))} />
          {value === 'true' ? 'Ligado' : 'Desligado'}
        </label>
      ) : f.kind === 'enum' ? (
        <select id={id} value={value} disabled={disabled} aria-describedby={helpId} onChange={(e) => onChange(e.target.value)}>
          {f.options?.map((o) => (
            <option key={o} value={o}>
              {WEEKDAY_LABEL[o] ?? o}
            </option>
          ))}
        </select>
      ) : f.kind === 'int' ? (
        <input id={id} type="number" min={f.min} max={f.max} value={value} disabled={disabled} aria-describedby={helpId} onChange={(e) => onChange(e.target.value)} />
      ) : f.kind === 'secret' ? (
        <input
          id={id}
          type="password"
          autoComplete="new-password"
          value={value}
          disabled={disabled}
          aria-describedby={helpId}
          placeholder={f.isSet ? 'Deixe vazio para manter o atual' : 'Cole o valor aqui'}
          onChange={(e) => onChange(e.target.value)}
        />
      ) : (
        <input id={id} type="text" value={value} disabled={disabled} aria-describedby={helpId} placeholder={f.default || undefined} onChange={(e) => onChange(e.target.value)} />
      )}

      <p className="tile-note" id={helpId}>
        {f.help}
        {f.kind === 'secret' && !encryption && ' Defina APP_SECRET_KEY no ambiente para guardar segredos aqui.'}
        {f.pendingRestart && ' Alterado: falta reiniciar o app para valer.'}
      </p>
      {f.source === 'db' && (
        <button type="button" className="link" disabled={busy} onClick={onReset}>
          Voltar ao valor do ambiente ou padrão
        </button>
      )}
    </div>
  )
}
