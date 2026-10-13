import { describe, expect, it } from 'vitest'
import type { APIKey, ProviderInfo } from '@/api/types.gen'
import { clampPrompt, modelsForKey, validateNewBook } from './new-book-logic'

const providers: ProviderInfo[] = [
  { name: 'openrouter', label: 'OpenRouter', models: [{ id: 'a', name: 'A', context_window: 1, input_cost_per_1m: 1, output_cost_per_1m: 2 }] },
  { name: 'gemini', label: 'Gemini', models: [] },
]
const key = (provider: string): APIKey => ({ id: 'k', provider, label: '', last4: '1234', created_at: '' })

describe('new book logic', () => {
  it('chỉ trả model của provider thuộc key', () => {
    expect(modelsForKey(key('openrouter'), providers).map((m) => m.id)).toEqual(['a'])
    expect(modelsForKey(key('gemini'), providers)).toEqual([])
    expect(modelsForKey(key('khong-co'), providers)).toEqual([])
    expect(modelsForKey(undefined, providers)).toEqual([])
  })

  it('kiểm tra từng trường', () => {
    const ok = { prompt: 'Một câu chuyện', keyId: 'k', model: 'a', budget: '5' }
    expect(validateNewBook(ok)).toEqual({})
    expect(validateNewBook({ ...ok, prompt: '   ' }).prompt).toBeTruthy()
    expect(validateNewBook({ ...ok, keyId: '' }).keyId).toBeTruthy()
    expect(validateNewBook({ ...ok, model: '' }).model).toBeTruthy()
    for (const b of ['', '0', '-1', 'abc', '51']) {
      expect(validateNewBook({ ...ok, budget: b }).budget).toBeTruthy()
    }
    expect(validateNewBook({ ...ok, budget: '2,5' }).budget).toBeUndefined()
  })

  it('cắt prompt theo ký tự, không cắt giữa cặp thay thế', () => {
    const s = '😀'.repeat(4001)
    expect([...clampPrompt(s)]).toHaveLength(4000)
  })
})
