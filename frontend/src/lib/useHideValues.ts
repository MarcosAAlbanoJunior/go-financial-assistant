import { useState } from 'react'

const KEY = 'hide-values'

// O armazenamento pode estar bloqueado (navegação privada): nunca deve quebrar a página.
function stored(): boolean {
  try {
    return localStorage.getItem(KEY) === '1'
  } catch {
    return false
  }
}

/** "Ocultar valores": só do navegador, começa visível e persiste ao recarregar. */
export function useHideValues() {
  const [hidden, setHidden] = useState(stored)

  function toggle() {
    const next = !hidden
    try {
      localStorage.setItem(KEY, next ? '1' : '0')
    } catch {
      // sem persistência, vale só nesta sessão
    }
    setHidden(next)
  }

  return { hidden, toggle }
}
