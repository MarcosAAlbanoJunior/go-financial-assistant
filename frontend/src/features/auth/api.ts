import { useQuery } from '@tanstack/react-query'
import { postJSON, request } from '../../shared/api/request'

export const login = (password: string) => postJSON('/api/login', { password })

export const logout = () => postJSON('/api/logout')

// Sem retry em 401/403: repetir não muda o resultado e só atrasa o redirecionamento ao login.
export const useMe = () =>
  useQuery({ queryKey: ['me'], queryFn: () => request('/api/me'), retry: false, staleTime: 60_000 })
