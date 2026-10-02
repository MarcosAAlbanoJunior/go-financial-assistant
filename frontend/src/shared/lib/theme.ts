import { useState } from 'react'

type Theme = 'light' | 'dark'
const KEY = 'theme'

// O armazenamento pode estar bloqueado (navegação privada, política do navegador): nunca deve quebrar a página.
function stored(): Theme | null {
  try {
    const v = localStorage.getItem(KEY)
    return v === 'light' || v === 'dark' ? v : null
  } catch {
    return null
  }
}

const systemTheme = (): Theme => (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')

/** Alterna claro/escuro. Sem escolha salva, vale a preferência do sistema. */
export function useTheme() {
  const [theme, setTheme] = useState<Theme>(() => stored() ?? systemTheme())

  function toggle() {
    const next: Theme = theme === 'dark' ? 'light' : 'dark'
    document.documentElement.dataset.theme = next
    try {
      localStorage.setItem(KEY, next)
    } catch {
      // sem persistência, o tema vale só nesta sessão
    }
    setTheme(next)
  }
  return { theme, toggle }
}

/** Aplica a escolha salva antes da primeira renderização, evitando o flash de tema errado. */
export function applyStoredTheme() {
  const t = stored()
  if (t) document.documentElement.dataset.theme = t
}
