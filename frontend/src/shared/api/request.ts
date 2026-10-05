export class ApiError extends Error {
  status: number
  /** O app ainda não foi configurado: a API só atende o setup ({"setup": "required"}) e o front leva para /setup. */
  setupRequired: boolean
  constructor(status: number, message: string, setupRequired = false) {
    super(message)
    this.status = status
    this.setupRequired = setupRequired
  }
}

export const isSetupRequired = (error: unknown) => error instanceof ApiError && error.setupRequired

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  // Mesma origem: o cookie de sessão (HttpOnly) vai sozinho, e o front nunca vê a senha nem o cookie.
  const res = await fetch(path, { credentials: 'same-origin', ...init })
  const body = await res.json().catch(() => null)
  if (!res.ok) throw new ApiError(res.status, body?.error ?? 'erro inesperado', body?.setup === 'required')
  return body as T
}

export const postJSON = <T>(path: string, data?: unknown) =>
  request<T>(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data ?? {}),
  })
