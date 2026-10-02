import { useState } from 'react'
import { useSearchParams } from 'react-router'
import { loadPremises, loadScenarios, savePremises, saveScenarios, type Premises, type Scenario } from './simulation'

export const RANGES = [6, 12, 24]

/**
 * Estado da tela de projeção: o período (na URL) e as premissas e cenários editados, que ficam salvos só neste navegador.
 */
export function useProjectionState() {
  const [params, setParams] = useSearchParams()
  const requested = Number(params.get('meses'))
  const months = RANGES.includes(requested) ? requested : 12
  const setMonths = (n: number) =>
    setParams((prev) => {
      const next = new URLSearchParams(prev)
      next.set('meses', String(n))
      return next
    })

  const [scenarios, setScenarios] = useState<Scenario[]>(loadScenarios)
  const updateScenarios = (next: Scenario[]) => {
    setScenarios(next)
    saveScenarios(next)
  }

  const [override, setOverride] = useState<Partial<Premises>>(loadPremises)
  const updateOverride = (next: Partial<Premises>) => {
    setOverride(next)
    savePremises(next)
  }

  return { months, setMonths, scenarios, updateScenarios, override, updateOverride }
}
