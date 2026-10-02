import { useEffect, useRef, useState } from 'react'

const DURATION_MS = 400

const reducedMotion = () => matchMedia('(prefers-reduced-motion: reduce)').matches

/** Número que sobe até o valor em ~400 ms (da última posição mostrada). Desligado com prefers-reduced-motion. */
export function useCountUp(value: number): number {
  const reduced = reducedMotion()
  const [shown, setShown] = useState(reduced ? value : 0)
  const current = useRef(shown)

  useEffect(() => {
    if (reduced) return
    const from = current.current
    const start = performance.now()
    let frame = requestAnimationFrame(function tick(now) {
      const t = Math.min(1, (now - start) / DURATION_MS)
      current.current = from + (value - from) * (1 - (1 - t) ** 3)
      setShown(current.current)
      if (t < 1) frame = requestAnimationFrame(tick)
    })
    return () => cancelAnimationFrame(frame)
  }, [value, reduced])

  return reduced ? value : shown
}
