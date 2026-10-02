import { useQuery } from '@tanstack/react-query'
import { request } from '../../shared/api/request'
import type { ReviewKind } from '../review/api'

/** O que o Coach envia à IA (as chaves seguem o JSON do servidor). */
export interface CoachContext {
  mes: string
  meses: string[]
  receita_do_mes: number
  despesa_do_mes: number
  categorias: { nome: string; gasto_por_mes: number[] }[]
  sugestoes: { id: string; tipo: string; nome: string; categoria: string; valor_no_mes: number; economia_possivel: number }[]
  metas: { id: string; tipo: string; nome: string; atual: number; alvo: number; atingida: boolean }[]
  /** Análises passadas e as respostas da pessoa, que a IA recebe para lembrar o que já foi dito. */
  memoria: { mes: string; resumo: string; respostas: { sobre?: string; pergunta: string; resposta: string }[] }[]
}

export interface CoachPreview {
  enabled: boolean
  blockedReason: string
  provider: string
  month: string
  bytes: number
  /** Identifica o contexto exibido: a análise só envia se ele ainda for o mesmo. */
  hash: string
  context: CoachContext
}

export interface CoachAction {
  suggestionId: string
  priority: number
  comment: string
  question: string
  /** Números calculados pelo código (nulo se o id não casar). */
  suggestion: {
    kind: ReviewKind
    key: string
    label: string
    category: string
    categoryLabel: string
    monthly: number
    annual: number | null
    amount: number
  } | null
}

export interface CoachRecord {
  summary: string
  actions: CoachAction[]
  goals: { goalId: string; name: string; comment: string }[]
  questions: string[]
}

/** Uma análise guardada. As chaves de `answers` são "a:s3" (sobre a sugestão s3) ou "q:0" (pergunta geral 0). */
export interface CoachAnalysis {
  id: string
  month: string
  createdAt: string
  advice: CoachRecord
  answers: Record<string, string>
}

/** Só lê: mostra o que o Coach enviaria, sem enviar nada. */
export const useCoachPreview = (month: string) =>
  useQuery({
    queryKey: ['coach-preview', month],
    queryFn: () => request<CoachPreview>(`/api/coach/preview?month=${month}`),
  })

/** Envia ao Gemini o contexto da prévia (o hash garante que é o que a pessoa viu); a análise já volta gravada. */
export const analyzeCoach = (month: string, hash: string) =>
  request<CoachAnalysis>('/api/coach/analyze', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ month, hash }),
  })

export const useCoachAnalyses = (month: string) =>
  useQuery({
    queryKey: ['coach-analyses', month],
    queryFn: () => request<CoachAnalysis[]>(`/api/coach/analyses?month=${month}`),
  })

/** Grava a resposta a uma pergunta da IA; vazia, apaga. */
export const setCoachAnswer = (id: string, key: string, answer: string) =>
  request(`/api/coach/analyses/${encodeURIComponent(id)}/answers`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, answer }),
  })

export const deleteCoachAnalysis = (id: string) => request(`/api/coach/analyses/${encodeURIComponent(id)}`, { method: 'DELETE' })
