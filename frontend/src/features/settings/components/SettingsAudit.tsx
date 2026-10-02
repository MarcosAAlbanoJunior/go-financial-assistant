import { auditText } from '../lib/settings'
import { useSettingsAudit } from '../api'

/** Quem mudou o quê (sem os valores). Mudanças sensíveis também chegam como aviso no chat. */
export function SettingsAudit() {
  const query = useSettingsAudit()
  const entries = query.data ?? []
  return (
    <section className="card set-card" aria-labelledby="g-audit">
      <h2 className="chart-title" id="g-audit">
        Histórico de alterações
      </h2>
      <p className="tile-note">As últimas mudanças feitas aqui, sem os valores. Mudanças sensíveis também avisam no seu chat.</p>
      {query.isError ? (
        <p className="error">Não foi possível carregar o histórico.</p>
      ) : entries.length === 0 ? (
        <p className="tile-note">Nenhuma alteração ainda.</p>
      ) : (
        <ul className="set-audit">
          {entries.map((e, i) => (
            <li key={i}>
              <time dateTime={e.at}>{new Date(e.at).toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' })}</time>
              <span>
                {auditText(e.action, e.label)}
                {e.sensitive && <span className="badge">sensível</span>}
              </span>
              <span className="tile-note">{e.ip}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
