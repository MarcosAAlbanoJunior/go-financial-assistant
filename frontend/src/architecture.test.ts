import { describe, expect, it } from 'vitest'

// Cada funcionalidade (features/<nome>) só pode importar dela mesma e de shared/. A única exceção é coach → review (o Coach
// comenta as sugestões da Revisão). Para compartilhar algo entre funcionalidades, mova para shared/.
const ALLOWED: Record<string, string[]> = { coach: ['review'] }

const sources = import.meta.glob('./features/**/*.{ts,tsx}', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

const IMPORT = /(?:from|import)\s+['"](\.{1,2}\/[^'"]+)['"]/g

/** Resolve um import relativo ao caminho do arquivo ("./features/a/b.ts" + "../c/d" -> "features/c/d"). */
function resolve(from: string, spec: string): string {
  const parts = from.replace(/^\.\//, '').split('/').slice(0, -1)
  for (const seg of spec.split('/')) {
    if (seg === '..') parts.pop()
    else if (seg !== '.') parts.push(seg)
  }
  return parts.join('/')
}

describe('arquitetura do frontend', () => {
  it('uma funcionalidade só importa dela mesma e de shared', () => {
    const violations: string[] = []
    for (const [file, text] of Object.entries(sources)) {
      const own = file.replace(/^\.\/features\//, '').split('/')[0]
      for (const m of text.matchAll(IMPORT)) {
        const target = resolve(file, m[1])
        const [root, feature] = target.split('/')
        if (root !== 'features' || feature === own) continue
        if (!(ALLOWED[own] ?? []).includes(feature)) violations.push(`${file} importa features/${feature}`)
      }
    }
    expect(violations).toEqual([])
  })

  it('shared não depende de nenhuma funcionalidade', () => {
    // O Layout é a exceção de composição? Não: ele só conhece rotas por caminho, nunca importa páginas.
    const shared = import.meta.glob('./shared/**/*.{ts,tsx}', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
    const violations: string[] = []
    for (const [file, text] of Object.entries(shared)) {
      for (const m of text.matchAll(IMPORT)) {
        if (resolve(file, m[1]).startsWith('features/')) violations.push(`${file} importa ${resolve(file, m[1])}`)
      }
    }
    expect(violations).toEqual([])
  })
})
