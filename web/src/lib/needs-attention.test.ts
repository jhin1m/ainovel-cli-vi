import { describe, expect, it } from 'vitest'
import { BOOK_STATUSES } from '@/api/status'
import type { Book } from '@/api/types.gen'
import { needsAttention } from './needs-attention'

const book = (id: string, status: string) => ({ id, status }) as Book

describe('needsAttention', () => {
  it('chỉ lấy truyện chờ duyệt, lỗi, hết ngân sách, bị ngắt', () => {
    const books = BOOK_STATUSES.map((s) => book(s, s))
    expect(needsAttention(books).map((b) => b.id)).toEqual([
      'waiting_review',
      'budget_exhausted',
      'failed',
      'interrupted',
    ])
  })

  it('danh sách rỗng hoặc trạng thái lạ thì không có gì', () => {
    expect(needsAttention([])).toEqual([])
    expect(needsAttention([book('x', 'xyz'), book('y', 'running')])).toEqual([])
  })
})
