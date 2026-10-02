import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Bot, Check, ShieldAlert } from 'lucide-react'
import { useState } from 'react'
import { ApiError, setCategoryRule, suggestCategories, useCategorize } from '../api/client'
import type { Categorize as CategorizeData } from '../api/types'
import { QueryState } from '../components/QueryState'
import { StatTile } from '../components/StatTile'
import { cleanLabel } from '../lib/budget'
import { groupSummary, pendingChoices, totalOf, withSuggestions } from '../lib/categorize'
import { formatBRL } from '../lib/format'

export default function Categorize() {
  const query = useCategorize()
  const queryClient = useQueryClient()
  // Escolhas ainda não aplicadas, por conta; e quais vieram da IA (só para sinalizar na tela).
  const [choice, setChoice] = useState<Record<string, string>>({})
  const [fromAI, setFromAI] = useState<Set<string>>(new Set())
  // O aceite vale para os dados exatos que a pessoa viu (o hash).
  const [agreedHash, setAgreedHash] = useState<string | null>(null)
  const [changed, setChanged] = useState<number | null>(null)

  const apply = useMutation({
    mutationFn: async (items: { key: string; category: string }[]) => {
      let total = 0
      for (const it of items) total += (await setCategoryRule(it.key, it.category)).changed
      return total
    },
    onSuccess: (total, items) => {
      setChanged(total)
      setChoice((prev) => Object.fromEntries(Object.entries(prev).filter(([k]) => !items.some((i) => i.key === k))))
      // A categoria muda em todas as telas.
      return queryClient.invalidateQueries()
    },
  })
  const suggest = useMutation({
    mutationFn: (hash: string) => suggestCategories(hash),
    onSuccess: ({ suggestions }) => {
      setChoice((prev) => withSuggestions(prev, suggestions))
      setFromAI((prev) => new Set([...prev, ...suggestions.map((s) => s.key)]))
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['categorize'] }),
  })

  return (
    <>
      <h1 className="page-title">Classificar contas em &quot;Outros&quot;</h1>
      <QueryState query={query}>
        {(data) => {
          const pending = pendingChoices(choice, data.groups)
          return (
            <>
              <div className="tiles">
                <StatTile
                  label="Contas em Outros"
                  value={String(data.groups.length)}
                  note="Da maior para a menor; o nome aparece só para você."
                />
                <StatTile label="Valor coberto pela lista" value={formatBRL(totalOf(data.groups))} />
              </div>
              <p className="tile-note section-gap">
                Escolha a categoria de cada conta e aplique: as despesas antigas dela mudam de categoria e as próximas sincronizações já
                chegam classificadas. &quot;Manter em Outros&quot; tira a conta da lista. Pix e transferências só você sabe do que se trata;
                a IA nunca os vê.
              </p>

              <AiCard data={data} agreedHash={agreedHash} setAgreedHash={setAgreedHash} suggest={suggest} />

              {(apply.isError || changed !== null) && (
                <p className={apply.isError ? 'state error' : 'state'} role={apply.isError ? 'alert' : 'status'}>
                  {apply.isError
                    ? 'Não foi possível aplicar. Tente de novo.'
                    : `${changed} ${changed === 1 ? 'lançamento reclassificado' : 'lançamentos reclassificados'}.`}
                </p>
              )}

              {data.groups.length === 0 ? (
                <p className="state">Nenhuma conta em Outros sem classificação.</p>
              ) : (
                <ul className="classify-list section-gap">
                  {data.groups.map((g) => (
                    <li key={g.key} className="classify-row">
                      <span className="classify-name">
                        <strong>{cleanLabel(g.label)}</strong>
                        {g.transfer && <span className="badge">Pix/transferência</span>}
                        {fromAI.has(g.key) && choice[g.key] && (
                          <span className="badge">
                            <Bot size={11} aria-hidden="true" /> sugestão da IA
                          </span>
                        )}
                        <span className="bill-meta">{groupSummary(g)}</span>
                      </span>
                      <label>
                        <span className="sr-only">Categoria de {cleanLabel(g.label)}</span>
                        <select value={choice[g.key] ?? ''} onChange={(e) => setChoice((prev) => ({ ...prev, [g.key]: e.target.value }))}>
                          <option value="">Escolher…</option>
                          {data.categories.map((c) => (
                            <option key={c.value} value={c.value}>
                              {c.value === 'OTHER' ? 'Manter em Outros' : c.label}
                            </option>
                          ))}
                        </select>
                      </label>
                      <button
                        type="button"
                        className="btn btn-small"
                        disabled={!choice[g.key] || apply.isPending}
                        onClick={() => apply.mutate([{ key: g.key, category: choice[g.key] }])}
                      >
                        Aplicar
                      </button>
                    </li>
                  ))}
                </ul>
              )}
              {pending.length > 1 && (
                <button
                  type="button"
                  className="btn btn-primary section-gap"
                  disabled={apply.isPending}
                  onClick={() => apply.mutate(pending)}
                >
                  <Check size={14} aria-hidden="true" /> Aplicar as {pending.length} escolhas
                </button>
              )}
            </>
          )
        }}
      </QueryState>
    </>
  )
}

function AiCard({
  data,
  agreedHash,
  setAgreedHash,
  suggest,
}: {
  data: CategorizeData
  agreedHash: string | null
  setAgreedHash: (h: string | null) => void
  suggest: { mutate: (hash: string) => void; isPending: boolean; isError: boolean; error: unknown; data?: { suggestions: unknown[] } }
}) {
  const { ai } = data
  if (ai.context.contas.length === 0) return null
  const agreed = agreedHash === ai.hash
  return (
    <section className="card section-gap" aria-labelledby="ai-title">
      <h2 className="chart-title" id="ai-title">
        Sugerir categorias com IA (comércio e serviços)
      </h2>
      <p className="chart-sub">
        Vão ao {ai.provider} só os nomes abaixo, com quantidade e total. Pix e transferências nunca vão. A IA só sugere: nada muda até você
        aplicar.
      </p>
      <ul className="coach-names">
        {ai.context.contas.map((c) => (
          <li key={c.id}>{c.nome}</li>
        ))}
      </ul>
      <details className="coach-json">
        <summary>Ver o JSON exato ({ai.bytes} bytes)</summary>
        <pre>{JSON.stringify(ai.context, null, 2)}</pre>
      </details>
      {ai.enabled ? (
        <>
          <label className="check">
            <input type="checkbox" checked={agreed} onChange={(e) => setAgreedHash(e.target.checked ? ai.hash : null)} />
            Entendo que os nomes acima serão enviados ao {ai.provider}.
          </label>
          <div className="form-actions section-gap">
            <button
              type="button"
              className="btn btn-primary"
              disabled={!agreed || suggest.isPending}
              onClick={() => suggest.mutate(ai.hash)}
            >
              <Bot size={14} aria-hidden="true" /> {suggest.isPending ? 'Pensando…' : 'Sugerir categorias'}
            </button>
            {suggest.data && <span className="tile-note">{suggest.data.suggestions.length} sugestões. Revise e aplique.</span>}
          </div>
          {suggest.isError && (
            <p className="state error" role="alert">
              {suggest.error instanceof ApiError ? suggest.error.message : 'Não foi possível sugerir. Tente de novo.'}
            </p>
          )}
        </>
      ) : (
        <p className="state error" role="alert">
          <ShieldAlert size={16} aria-hidden="true" /> {ai.blockedReason}
        </p>
      )}
    </section>
  )
}
