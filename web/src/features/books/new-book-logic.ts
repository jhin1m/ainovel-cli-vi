import type { APIKey, ModelOption, ProviderInfo } from '@/api/types.gen'
import { msg } from '@/lib/messages'

export const MAX_PROMPT_RUNES = 4000
export const MAX_BUDGET_USD = 50
export const DEFAULT_BUDGET_USD = 5

export interface NewBookInput {
  prompt: string
  keyId: string
  model: string
  budget: string
}

export type NewBookErrors = Partial<Record<keyof NewBookInput, string>>

export function runeLength(s: string): number {
  return [...s].length
}

export function clampPrompt(s: string): string {
  const r = [...s]
  return r.length > MAX_PROMPT_RUNES ? r.slice(0, MAX_PROMPT_RUNES).join('') : s
}

/** Model chọn được cho một key: chỉ của provider đó (server đã lọc model có giá). */
export function modelsForKey(key: APIKey | undefined, providers: ProviderInfo[]): ModelOption[] {
  if (!key) return []
  return providers.find((p) => p.name === key.provider)?.models ?? []
}

export function parseBudget(raw: string): number {
  return Number(raw.replace(',', '.'))
}

/** Ngân sách mới khi tăng: phải vượt số đã dùng và không quá trần; ngược lại trả null. */
export function parseRaisedBudget(raw: string, spent: number): number | null {
  const v = parseBudget(raw)
  if (raw.trim() === '' || !Number.isFinite(v) || v <= spent || v > MAX_BUDGET_USD) return null
  return v
}

export function validateNewBook(v: NewBookInput): NewBookErrors {
  const errors: NewBookErrors = {}
  const prompt = v.prompt.trim()
  if (prompt === '') errors.prompt = msg.newBook.promptRequired
  else if (runeLength(prompt) > MAX_PROMPT_RUNES) errors.prompt = msg.newBook.promptTooLong
  if (v.keyId === '') errors.keyId = msg.newBook.keyRequired
  if (v.model === '') errors.model = msg.newBook.modelRequired
  const budget = parseBudget(v.budget)
  if (v.budget.trim() === '' || !Number.isFinite(budget) || !(budget > 0) || budget > MAX_BUDGET_USD) {
    errors.budget = msg.newBook.budgetInvalid(MAX_BUDGET_USD)
  }
  return errors
}
