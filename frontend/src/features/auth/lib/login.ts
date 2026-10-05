import { ApiError } from '../../../shared/api/request'

/** A mensagem de erro de cada passo do login. As do servidor (código expirado, errado, canal fora) já vêm prontas. */
export function loginErrorMessage(error: unknown, step: 'password' | 'code'): string | null {
  if (!error) return null
  if (!(error instanceof ApiError)) return 'Não foi possível entrar. Tente de novo.'
  if (error.status === 401 && step === 'password') return 'Senha incorreta.'
  // O limite por IP responde sem JSON, então chega com a mensagem genérica.
  if (error.status === 429 && error.message === 'erro inesperado') return 'Muitas tentativas. Aguarde um minuto e tente de novo.'
  if (error.message === 'erro inesperado') return 'Não foi possível entrar. Tente de novo.'
  return error.message.charAt(0).toUpperCase() + error.message.slice(1) + '.'
}
