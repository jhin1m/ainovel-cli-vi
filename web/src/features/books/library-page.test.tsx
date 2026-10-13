import { screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { APIKey, Book, BookSnapshot } from '@/api/types.gen'
import { jsonResponse, renderWithProviders } from '@/test/render'
import { LibraryPage } from './library-page'

const fetchMock = vi.fn<typeof fetch>()

beforeEach(() => vi.stubGlobal('fetch', fetchMock))
afterEach(() => {
  vi.unstubAllGlobals()
  fetchMock.mockReset()
})

const key: APIKey = { id: 'k1', provider: 'zai', label: '', last4: 'ab12', created_at: '2026-10-13T00:00:00Z' }

function book(id: string, status: string, completed = 1): Book {
  return {
    id,
    prompt: 'Truyện ' + id,
    status,
    stop_reason: '',
    provider: 'zai',
    model: 'glm-4.7',
    api_key_id: 'k1',
    budget_usd: 2,
    subscription: false,
    advance_mode: '',
    pending_text: '',
    last_error: '',
    snapshot: { book_title: 'Truyện ' + id, completed_count: completed, total_chapters: 10 } as BookSnapshot,
    created_at: '2026-10-10T00:00:00Z',
    updated_at: '2026-10-10T00:00:00Z',
  }
}

function serve(keys: APIKey[], books: Book[]) {
  fetchMock.mockImplementation((input) => {
    const url = String(input)
    if (url === '/api/keys') return Promise.resolve(jsonResponse(keys))
    if (url === '/api/books') return Promise.resolve(jsonResponse(books))
    return Promise.resolve(jsonResponse({ error: { code: 'nf', message: 'không có' } }, 404))
  })
}

describe('LibraryPage', () => {
  it('user mới: checklist bước 1, empty state và khối chạy local', async () => {
    serve([], [])
    await renderWithProviders(<LibraryPage />)
    expect(await screen.findByRole('heading', { name: 'Bắt đầu trong 3 bước' })).toBeInTheDocument()
    expect(document.querySelector('[data-step="key"]')).toHaveAttribute('aria-current', 'step')
    expect(screen.getByText('Bạn chưa có truyện nào.')).toBeInTheDocument()
    expect(screen.getByText('Chạy trên máy bạn')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: /Cần bạn xử lý/ })).toBeNull()
  })

  it('có truyện: dải "Cần bạn xử lý" chỉ gồm trạng thái cần xử lý, lưới thẻ, ẩn checklist khi xong', async () => {
    serve(
      [key],
      [
        book('a', 'running'),
        book('b', 'waiting_review'),
        book('c', 'failed'),
        book('d', 'completed'),
        book('e', 'budget_exhausted'),
        book('f', 'interrupted'),
        book('g', 'paused'),
      ],
    )
    await renderWithProviders(<LibraryPage />)
    const strip = (await screen.findByRole('heading', { name: 'Cần bạn xử lý (4)' })).closest('section')!
    const rows = within(strip).getAllByRole('listitem').map((li) => li.textContent)
    expect(rows).toHaveLength(4)
    for (const id of ['b', 'c', 'e', 'f']) expect(rows.some((r) => r?.includes('Truyện ' + id))).toBe(true)
    expect(within(strip).getByRole('link', { name: 'Duyệt chương: Truyện b' })).toHaveAttribute('href', '/books/b')
    // Nút ngân sách / chạy lại mở trang truyện kèm hành động; "Xem lỗi" chỉ mở trang.
    expect(within(strip).getByRole('link', { name: 'Tăng ngân sách: Truyện e' })).toHaveAttribute('href', '/books/e?action=budget')
    expect(within(strip).getByRole('link', { name: 'Chạy lại: Truyện f' })).toHaveAttribute('href', '/books/f?action=retry')
    expect(within(strip).getByRole('link', { name: 'Xem lỗi: Truyện c' })).toHaveAttribute('href', '/books/c')
    expect(screen.queryByRole('heading', { name: 'Bắt đầu trong 3 bước' })).toBeNull()
    expect(screen.getAllByRole('article')).toHaveLength(7)
    expect(screen.getByRole('link', { name: 'Viết truyện mới' })).toHaveAttribute('href', '/books/new')
  })
})
