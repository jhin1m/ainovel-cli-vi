import { describe, expect, it } from 'vitest'
import type { APIKey, Book, BookSnapshot, ModelOption, ProviderInfo } from '@/api/types.gen'
import {
  EMPTY_PROMPT_OPTIONS,
  clampPrompt,
  composePrompt,
  estimateChapters,
  modelsForKey,
  pickDefaultModel,
  priceTiers,
  validateNewBook,
} from './new-book-logic'

const providers: ProviderInfo[] = [
  {
    name: 'openrouter',
    label: 'OpenRouter',
    subscription: false,
    models: [{ id: 'a', name: 'A', context_window: 1, input_cost_per_1m: 1, output_cost_per_1m: 2, recommended: false }],
  },
  { name: 'gemini', label: 'Gemini', subscription: false, models: [] },
]
const key = (provider: string): APIKey => ({ id: 'k', provider, label: '', last4: '1234', created_at: '' })

const m = (id: string, input: number, output: number, recommended = false): ModelOption => ({
  id,
  name: id.toUpperCase(),
  context_window: 1,
  input_cost_per_1m: input,
  output_cost_per_1m: output,
  recommended,
})

const book = (model: string, cost: number, done: number, subscription = false): Book => ({
  id: `${model}-${cost}-${done}`,
  prompt: '',
  status: 'completed',
  stop_reason: '',
  provider: 'openrouter',
  model,
  api_key_id: 'k',
  budget_usd: 5,
  subscription,
  advance_mode: 'review',
  pending_text: '',
  last_error: '',
  snapshot: { total_cost_usd: cost, completed_count: done } as BookSnapshot,
  created_at: '',
  updated_at: '',
})

describe('new book logic', () => {
  it('chỉ trả model của provider thuộc key', () => {
    expect(modelsForKey(key('openrouter'), providers).map((x) => x.id)).toEqual(['a'])
    expect(modelsForKey(key('gemini'), providers)).toEqual([])
    expect(modelsForKey(key('khong-co'), providers)).toEqual([])
    expect(modelsForKey(undefined, providers)).toEqual([])
  })

  it('kiểm tra từng trường', () => {
    const ok = { prompt: 'Một câu chuyện', keyId: 'k', model: 'a', budget: '5' }
    expect(validateNewBook(ok)).toEqual({})
    expect(validateNewBook({ ...ok, prompt: '   ' }).prompt).toBeTruthy()
    expect(validateNewBook({ ...ok, prompt: 'x'.repeat(4001) }).prompt).toBeTruthy()
    expect(validateNewBook({ ...ok, keyId: '' }).keyId).toBeTruthy()
    expect(validateNewBook({ ...ok, model: '' }).model).toBeTruthy()
    for (const b of ['', '0', '-1', 'abc', '51']) {
      expect(validateNewBook({ ...ok, budget: b }).budget).toBeTruthy()
    }
    expect(validateNewBook({ ...ok, budget: '2,5' }).budget).toBeUndefined()
  })

  it('gói thuê bao bỏ qua ô ngân sách', () => {
    expect(validateNewBook({ prompt: 'x', keyId: 'k', model: 'a', budget: '' }, { subscription: true })).toEqual({})
  })

  it('cắt prompt theo ký tự, không cắt giữa cặp thay thế', () => {
    const s = '😀'.repeat(4001)
    expect([...clampPrompt(s)]).toHaveLength(4000)
  })
})

describe('composePrompt', () => {
  it('không có tuỳ chỉnh thì chỉ là ý tưởng đã trim', () => {
    expect(composePrompt('  Kiếm khách mù  ', EMPTY_PROMPT_OPTIONS)).toBe('Kiếm khách mù')
  })

  it('ý tưởng trống thì trả rỗng dù có tuỳ chỉnh', () => {
    expect(composePrompt('  ', { ...EMPTY_PROMPT_OPTIONS, genre: 'Tu tiên' })).toBe('')
  })

  it('ghép đủ các mục theo thứ tự, bỏ mục trống', () => {
    const out = composePrompt('Kiếm khách mù', {
      genre: 'Tu tiên',
      length: 'short',
      tone: 'U tối',
      protagonist: '  Lý Mặc  ',
      setting: '',
    })
    expect(out).toBe(
      'Kiếm khách mù\n\nThể loại: tu tiên.\nĐộ dài mong muốn: khoảng 5–10 chương.\nGiọng văn: u tối.\nNhân vật chính: Lý Mặc.',
    )
  })
})

describe('pickDefaultModel', () => {
  it('ưu tiên model đề xuất, không có thì model đầu', () => {
    expect(pickDefaultModel([m('a', 1, 1), m('b', 1, 1, true)])?.id).toBe('b')
    expect(pickDefaultModel([m('a', 1, 1), m('b', 1, 1)])?.id).toBe('a')
    expect(pickDefaultModel([])).toBeUndefined()
  })
})

describe('priceTiers', () => {
  it('dưới 3 model có giá thì không có bậc', () => {
    expect(priceTiers([m('a', 1, 1)])).toBeNull()
    expect(priceTiers([m('a', 1, 1), m('b', 2, 2), m('c', 0, 0)])).toBeNull()
  })

  it('mọi model cùng giá thì không có bậc', () => {
    expect(priceTiers([m('a', 1, 1), m('b', 1, 1), m('c', 1, 1)])).toBeNull()
  })

  it('chia ba bậc theo giá tương đối, cùng giá cùng bậc', () => {
    const t = priceTiers([m('c', 5, 10), m('a', 0.1, 0.4), m('b', 1, 2), m('b2', 1, 2)])
    expect(Object.fromEntries(t ?? [])).toEqual({ a: 'cheap', b: 'balanced', b2: 'balanced', c: 'quality' })
  })

  it('hai mức giá thì là rẻ và chất lượng', () => {
    const t = priceTiers([m('a', 1, 1), m('b', 1, 1), m('c', 3, 3)])
    expect(Object.fromEntries(t ?? [])).toEqual({ a: 'cheap', b: 'cheap', c: 'quality' })
  })
})

describe('estimateChapters', () => {
  it('chưa có số đo thì trả null', () => {
    expect(estimateChapters(5, [], 'x')).toBeNull()
    expect(estimateChapters(5, [book('x', 0, 0)], 'x')).toBeNull()
    expect(estimateChapters(5, [book('y', 1, 2)], 'x')).toBeNull()
  })

  it('tính từ tổng chi phí / tổng chương của truyện cùng model', () => {
    // 2 truyện: 1,5 US$ / 6 chương = 0,25 US$/chương → 5 US$ ≈ 20 chương, X = 15.
    expect(estimateChapters(5, [book('x', 1, 4), book('x', 0.5, 2), book('y', 9, 1)], 'x')).toEqual({
      min: 15,
      max: 20,
    })
  })

  it('bỏ truyện thuê bao cùng model (chi phí 0 làm ước tính sai)', () => {
    const books = [book('x', 1, 4), book('x', 0, 40, true)]
    expect(estimateChapters(5, books, 'x')).toEqual({ min: 15, max: 20 })
    expect(estimateChapters(5, [book('x', 0, 40, true)], 'x')).toBeNull()
  })

  it('ngân sách không hợp lệ thì trả null', () => {
    expect(estimateChapters(0, [book('x', 1, 4)], 'x')).toBeNull()
    expect(estimateChapters(Number.NaN, [book('x', 1, 4)], 'x')).toBeNull()
  })
})
