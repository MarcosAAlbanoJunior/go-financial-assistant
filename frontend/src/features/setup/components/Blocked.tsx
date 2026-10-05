import type { ProblemKind } from '../api'

/** O que falta no servidor antes de o setup começar. Nada funciona até resolver e reiniciar o app. */
export function Blocked({ kind, message, minLength }: { kind: ProblemKind; message: string; minLength: number }) {
  return (
    <div className="setup-step">
      <h2 className="chart-title">Falta um passo no servidor</h2>
      <p className="setup-lead">{message.charAt(0).toUpperCase() + message.slice(1)}.</p>
      {(kind === 'no-token' || kind === 'short-token') && (
        <>
          <p>Na pasta do projeto, no computador onde o app roda:</p>
          <ol className="setup-howto">
            <li>
              Rode <code>make init</code>. Ele cria o <code>.env</code>, a chave mestra e um <code>SETUP_TOKEN</code> aleatório (sem
              sobrescrever nada que já exista).
            </li>
            <li>
              Sem o <code>make</code>: gere um token com <code>openssl rand -hex 16</code> e coloque no <code>.env</code> como{' '}
              <code>SETUP_TOKEN=…</code> (mínimo de {minLength} caracteres).
            </li>
            <li>
              Reinicie: <code>docker compose up -d</code>. Depois recarregue esta página.
            </li>
          </ol>
        </>
      )}
      {kind === 'no-key' && (
        <ol className="setup-howto">
          <li>
            Rode <code>make secret-key</code> (cria <code>secrets/app_secret_key</code>).
          </li>
          <li>
            No <code>.env</code>, defina <code>APP_SECRET_KEY_FILE=/run/secrets/app_secret_key</code>.
          </li>
          <li>
            Reinicie com <code>docker compose up -d</code> e recarregue esta página. A chave cifra o token do bot no banco.
          </li>
        </ol>
      )}
      {kind === 'database' && (
        <ol className="setup-howto">
          <li>
            O banco foi criado antes desta versão. Aplique a migration nova:
            <pre className="setup-code">docker compose exec -T postgres psql -U finassist -d finassist {'<'} backend/migrations/019_create_dashboard_owner.sql</pre>
          </li>
          <li>
            Reinicie o app (<code>docker compose restart app</code>) e recarregue esta página.
          </li>
        </ol>
      )}
    </div>
  )
}
