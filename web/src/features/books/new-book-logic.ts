import type { APIKey, Book, ModelOption, ProviderInfo } from '@/api/types.gen'
import { msg } from '@/lib/messages'

export const MAX_PROMPT_RUNES = 4000
export const MAX_BUDGET_USD = 50
export const DEFAULT_BUDGET_USD = 5
export const BUDGET_CHIPS = [2, 5, 10, 20] as const

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

/**
 * Kiểm form. `prompt` là nội dung đã ghép (ý tưởng + tuỳ chỉnh thêm). Gói thuê bao
 * không có ngân sách USD nên bỏ qua ô ngân sách.
 */
export function validateNewBook(v: NewBookInput, opts: { subscription?: boolean } = {}): NewBookErrors {
  const errors: NewBookErrors = {}
  const prompt = v.prompt.trim()
  if (prompt === '') errors.prompt = msg.newBook.promptRequired
  else if (runeLength(prompt) > MAX_PROMPT_RUNES) errors.prompt = msg.newBook.promptTooLong
  if (v.keyId === '') errors.keyId = msg.newBook.keyRequired
  if (v.model === '') errors.model = msg.newBook.modelRequired
  const budget = parseBudget(v.budget)
  if (
    !opts.subscription &&
    (v.budget.trim() === '' || !Number.isFinite(budget) || !(budget > 0) || budget > MAX_BUDGET_USD)
  ) {
    errors.budget = msg.newBook.budgetInvalid(MAX_BUDGET_USD)
  }
  return errors
}

export type StoryLength = 'short' | 'medium' | 'long'

/** Phần "Tuỳ chỉnh thêm": mọi trường đều tuỳ chọn, rỗng thì không ghép. */
export interface PromptOptions {
  genre: string
  length: StoryLength | ''
  tone: string
  protagonist: string
  setting: string
}

export const EMPTY_PROMPT_OPTIONS: PromptOptions = { genre: '', length: '', tone: '', protagonist: '', setting: '' }

/**
 * Ghép ý tưởng với tuỳ chỉnh thêm thành đoạn gửi cho AI (API không có trường riêng).
 * Ý tưởng trống thì trả chuỗi rỗng để form báo thiếu ý tưởng.
 */
export function composePrompt(idea: string, o: PromptOptions): string {
  const base = idea.trim()
  if (base === '') return ''
  const t = msg.newBook.compose
  const lines = [
    o.genre && t.genre(o.genre),
    o.length && t.length(msg.newBook.lengths[o.length].detail),
    o.tone && t.tone(o.tone),
    o.protagonist.trim() && t.protagonist(o.protagonist.trim()),
    o.setting.trim() && t.setting(o.setting.trim()),
  ].filter(Boolean)
  return lines.length === 0 ? base : `${base}\n\n${lines.join('\n')}`
}

/** Model chọn sẵn khi đổi key/provider: model `recommended`, không có thì model đầu. */
export function pickDefaultModel(models: ModelOption[]): ModelOption | undefined {
  return models.find((m) => m.recommended) ?? models[0]
}

export type PriceTier = 'cheap' | 'balanced' | 'quality'

/**
 * Bậc giá tương đối trong cùng provider theo `input + output`. Xếp hạng trên các mức
 * giá khác nhau (cùng giá thì cùng bậc), chia ba khoảng đều: hạng/(số mức − 1) < 1/3 là
 * rẻ, ≤ 2/3 là cân bằng, còn lại là chất lượng. Trả null khi provider có dưới 3 model
 * có giá hoặc mọi model cùng giá (nhãn không có ý nghĩa).
 */
export function priceTiers(models: ModelOption[]): Map<string, PriceTier> | null {
  const cost = (m: ModelOption) => m.input_cost_per_1m + m.output_cost_per_1m
  const priced = models.filter((m) => cost(m) > 0)
  if (priced.length < 3) return null
  const levels = [...new Set(priced.map(cost))].sort((a, b) => a - b)
  const span = levels.length - 1
  if (span === 0) return null
  const tiers = new Map<string, PriceTier>()
  for (const m of priced) {
    const r = levels.indexOf(cost(m))
    tiers.set(m.id, r * 3 < span ? 'cheap' : r * 3 <= span * 2 ? 'balanced' : 'quality')
  }
  return tiers
}

/**
 * Ước tính số chương mà ngân sách đủ viết, chỉ dựa trên số đo thật: chi phí mỗi chương
 * = Σ chi phí / Σ chương đã xong của các truyện cùng model, bỏ truyện thuê bao (chi phí
 * USD luôn 0). Y = ngân sách / chi phí mỗi chương, X = 0,75·Y. Chưa có số đo hoặc
 * ngân sách không hợp lệ thì trả null.
 */
export function estimateChapters(
  budget: number,
  books: Book[],
  model: string,
): { min: number; max: number } | null {
  if (!Number.isFinite(budget) || !(budget > 0)) return null
  let cost = 0
  let chapters = 0
  for (const b of books) {
    if (b.model !== model || b.subscription || !b.snapshot) continue
    cost += b.snapshot.total_cost_usd
    chapters += b.snapshot.completed_count
  }
  if (chapters <= 0 || !(cost > 0)) return null
  const y = budget / (cost / chapters)
  const max = Math.floor(y)
  return { min: Math.min(max, Math.round(0.75 * y)), max }
}
