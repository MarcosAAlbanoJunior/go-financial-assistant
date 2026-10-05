import { ApiError } from '../../../shared/api/request'
import type { SetupStatus, TelegramState } from '../api'

export type Step = 'token' | 'password' | 'channel' | 'done'

export const STEPS: { id: Step; label: string }[] = [
  { id: 'token', label: 'Token' },
  { id: 'password', label: 'Senha' },
  { id: 'channel', label: 'Canal' },
  { id: 'done', label: 'Pronto' },
]

export const stepNumber = (step: Step) => STEPS.findIndex((s) => s.id === step) + 1

/** A senha está resolvida: nova (salva no rascunho), do .env, ou a de antes (setup reaberto). */
export const passwordReady = (s: SetupStatus) => !!s.password && (s.password.draft || s.password.fromEnv || s.password.current)

/** O passo em que a pessoa está, pelo estado salvo no servidor (voltar depois retoma do mesmo ponto). */
export function currentStep(s: SetupStatus, editingPassword = false): Step {
  if (!s.session) return 'token'
  if (editingPassword || !passwordReady(s)) return 'password'
  return 'channel'
}

export type TelegramStage = 'bot' | 'start' | 'confirm-person' | 'code'

/** Onde está o Telegram: colar o token do bot, esperar o /start, confirmar quem mandou, digitar o código. */
export function telegramStage(t: TelegramState | undefined): TelegramStage {
  if (!t?.bot) return 'bot'
  if (!t.candidate) return 'start'
  if (!t.accepted) return 'confirm-person'
  return 'code'
}

export interface Strength {
  score: 0 | 1 | 2 | 3
  label: string
}

/** Medidor simples: o mínimo é o tamanho; variedade e tamanho extra só melhoram a nota. */
export function passwordStrength(password: string, min: number): Strength {
  const length = [...password].length
  if (length < min) return { score: 0, label: `Curta: faltam ${min - length} caractere${min - length === 1 ? '' : 's'}` }
  const kinds = [/[a-z]/, /[A-Z]/, /\d/, /[^A-Za-z0-9]/].filter((re) => re.test(password)).length
  if (length >= 20 || (length >= 16 && kinds >= 3)) return { score: 3, label: 'Ótima' }
  if (length >= 16 || kinds >= 3) return { score: 2, label: 'Boa' }
  return { score: 1, label: 'Aceitável' }
}

/** Mensagem de erro para a tela. As do servidor já vêm em português; só ganham maiúscula e ponto. */
export function setupErrorMessage(error: unknown): string | null {
  if (!error) return null
  if (!(error instanceof ApiError)) return 'Não foi possível falar com o app. Ele está no ar?'
  // O limite por IP responde sem JSON.
  if (error.status === 429 && error.message === 'erro inesperado') return 'Muitas tentativas. Aguarde um minuto e tente de novo.'
  if (error.message === 'erro inesperado') return 'Algo deu errado. Tente de novo.'
  const m = error.message
  return m.charAt(0).toUpperCase() + m.slice(1) + (/[.!?]$/.test(m) ? '' : '.')
}

/** Quanto tempo a tela espera o /start antes de oferecer tentar de novo. */
export const START_WAIT_MS = 5 * 60_000
export const POLL_EVERY_MS = 2_000

export interface NextStep {
  title: string
  gain: string
  href: string
}

/** Próximos passos opcionais, cada um levando ao grupo certo das Configurações. */
export const NEXT_STEPS: NextStep[] = [
  {
    title: 'Gemini (opcional)',
    gain: 'Entender mensagens livres no chat, ler recibos e extratos, o Coach e as sugestões de categoria. Sem ele, o chat funciona com os comandos (/saldos, /resumo, /sync).',
    href: '/configuracoes#g-ai',
  },
  { title: 'Open Finance (Meu Pluggy)', gain: 'Sincronizar bancos e cartões sozinho, sem digitar nada.', href: '/configuracoes#g-pluggy' },
  { title: 'Seus nomes', gain: 'Ignorar Pix e transferências entre contas suas, que não são gasto nem renda.', href: '/configuracoes#g-sync' },
  { title: 'Resumo semanal', gain: 'Escolher o dia e a hora do resumo que chega no chat.', href: '/configuracoes#g-digest' },
]
