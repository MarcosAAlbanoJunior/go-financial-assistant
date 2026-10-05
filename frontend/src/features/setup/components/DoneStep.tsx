import { Link } from 'react-router'
import { NEXT_STEPS } from '../lib/setup'

/** Passo 4: pronto. O canal já está ligado e a sessão do dashboard aberta; o resto é opcional. */
export function DoneStep({ reopen, bot }: { reopen: boolean; bot: string }) {
  return (
    <div className="setup-step">
      <h2 className="chart-title">Pronto!</h2>
      <p className="setup-lead">
        {bot ? (
          <>
            O bot @{bot} já está respondendo no Telegram. Mande <code>/saldos</code> para testar.
          </>
        ) : (
          'O canal de conversa está ligado.'
        )}{' '}
        A partir de agora, o login pede a senha e um código no seu chat.
      </p>

      <h3 className="setup-subtitle">Próximos passos (opcionais)</h3>
      <ul className="setup-next">
        {NEXT_STEPS.map((n) => (
          <li key={n.href}>
            <Link to={n.href}>{n.title}</Link>
            <span className="tile-note">{n.gain}</span>
          </li>
        ))}
      </ul>

      <p className="setup-note">
        {reopen
          ? 'Remova SETUP_REOPEN do .env (enquanto ele estiver lá, nada mais acontece).'
          : 'Você já pode apagar o SETUP_TOKEN do .env (ou deixar: ele não abre mais nada).'}
      </p>
      <Link className="btn btn-primary setup-cta" to="/">
        Ir para o dashboard
      </Link>
    </div>
  )
}
