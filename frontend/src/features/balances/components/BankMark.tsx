import { useState } from 'react'
import { inkOn, monogram, safeHex } from '../lib/balances'

interface Props {
  name: string
  color: string | null
  logo: string | null
  size?: 'md' | 'sm'
}

/** Logo do banco (servido pela própria API) ou, sem ele, o monograma sobre a cor de marca. O nome vem sempre ao lado. */
export function BankMark({ name, color, logo, size = 'md' }: Props) {
  const [failed, setFailed] = useState(false)
  const brand = safeHex(color)

  if (logo && !failed) {
    return (
      <span className={`bal-mark bal-mark-${size} bal-mark-logo`} aria-hidden="true">
        <img src={logo} alt="" onError={() => setFailed(true)} />
      </span>
    )
  }
  return (
    <span className={`bal-mark bal-mark-${size}`} style={{ background: brand, color: inkOn(brand) }} aria-hidden="true">
      {monogram(name)}
    </span>
  )
}
