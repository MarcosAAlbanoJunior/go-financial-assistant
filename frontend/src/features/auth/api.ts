import { useQuery } from '@tanstack/react-query'
import { postJSON, request } from '../../shared/api/request'

/** Com o segundo fator ativo, a senha certa não abre a sessão: o código vai ao chat ({ step: 'code' }). */
export interface LoginResult {
  ok?: boolean
  step?: 'code'
}

export const login = (password: string) => postJSON<LoginResult>('/api/login', { password })

export const loginCode = (code: string) => postJSON<LoginResult>('/api/login/code', { code })

export const logout = () => postJSON('/api/logout')

export interface Me {
  ok: boolean
  secondFactor: boolean
}

// Sem retry em 401/403: repetir não muda o resultado e só atrasa o redirecionamento ao login.
export const useMe = () =>
  useQuery({ queryKey: ['me'], queryFn: () => request<Me>('/api/me'), retry: false, staleTime: 60_000 })
