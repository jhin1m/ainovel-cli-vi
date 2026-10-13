import { describe, expect, it } from 'vitest'
import { coverTone } from './cover-tone'

describe('coverTone', () => {
  it('cùng id luôn ra cùng tông', () => {
    expect(coverTone('book-abc')).toBe(coverTone('book-abc'))
  })

  it('luôn nằm trong 1..6, kể cả chuỗi rỗng', () => {
    for (const id of ['', 'a', '0f9c2d4e-1b7a-4c33-9e21-5d8f6a0b1c2d', 'Truyện tiên hiệp']) {
      const t = coverTone(id)
      expect(t).toBeGreaterThanOrEqual(1)
      expect(t).toBeLessThanOrEqual(6)
      expect(Number.isInteger(t)).toBe(true)
    }
  })

  it('rải đủ 6 tông trên nhiều id', () => {
    const seen = new Set(Array.from({ length: 60 }, (_, i) => coverTone(`book-${i}`)))
    expect(seen.size).toBe(6)
  })
})
