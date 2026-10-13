import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Book, BookSnapshot } from '@/api/types.gen'
import { setLastRead } from '@/lib/last-read'
import { jsonResponse, renderWithProviders } from '@/test/render'
import { BookCard, quickActions, shortModel } from './book-card'

const fetchMock = vi.fn<typeof fetch>()

beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock)
})
afterEach(() => {
  vi.unstubAllGlobals()
  fetchMock.mockReset()
  localStorage.clear()
})

function snap(over: Partial<BookSnapshot> = {}): BookSnapshot {
  return {
    book_title: 'Kiếm Đạo',
    completed_count: 12,
    total_chapters: 40,
    total_word_count: 38200,
    total_cost_usd: 1.5,
    total_input_tokens: 100000,
    total_output_tokens: 23456,
    ...over,
  } as BookSnapshot
}

function book(over: Partial<Book> = {}): Book {
  return {
    id: 'b1',
    prompt: 'Một truyện tiên hiệp',
    status: 'running',
    stop_reason: '',
    provider: 'openrouter',
    model: 'anthropic/claude-sonnet-4',
    api_key_id: 'k1',
    budget_usd: 5,
    subscription: false,
    advance_mode: '',
    pending_text: '',
    last_error: '',
    snapshot: snap(),
    created_at: '2026-10-01T00:00:00Z',
    updated_at: '2026-10-01T00:00:00Z',
    ...over,
  }
}

const NESTED = 'a a, a button, button a, button button'

describe('BookCard', () => {
  it('thẻ chờ duyệt có 2 nút và không lồng phần tử tương tác', async () => {
    const { container } = await renderWithProviders(<BookCard book={book({ status: 'waiting_review' })} />)
    expect(screen.getByRole('link', { name: 'Đọc chương 12' })).toHaveAttribute('href', '/books/b1/read/12')
    expect(screen.getByRole('button', { name: 'Viết tiếp' })).toBeInTheDocument()
    expect(container.querySelector(NESTED)).toBeNull()
  })

  it('"Viết tiếp" gọi đúng lệnh next như trang truyện', async () => {
    const updated = book({ status: 'running' })
    fetchMock.mockImplementation((input, init) => {
      if (String(input) === '/api/books/b1/next' && init?.method === 'POST') {
        return Promise.resolve(jsonResponse({ book: updated }))
      }
      return Promise.resolve(jsonResponse([]))
    })
    const user = userEvent.setup()
    await renderWithProviders(<BookCard book={book({ status: 'waiting_review' })} />)
    await user.click(screen.getByRole('button', { name: 'Viết tiếp' }))
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith('/api/books/b1/next', expect.objectContaining({ method: 'POST' })),
    )
    // Thẻ vẫn ở trạng thái chờ duyệt tới lượt poll sau: không cho bấm lại (server sẽ trả 409).
    await waitFor(() => expect(screen.getByRole('button', { name: 'Viết tiếp' })).toBeDisabled())
  })

  it('tiêu đề là link duy nhất tới trang truyện; có bìa, số liệu lớn, vòng tiến độ, icon provider', async () => {
    const { container } = await renderWithProviders(<BookCard book={book()} />)
    const title = screen.getByRole('link', { name: /Mở truyện Kiếm Đạo/ })
    expect(title).toHaveAttribute('href', '/books/b1')
    expect(title).toHaveTextContent('Kiếm Đạo')
    expect(container.querySelectorAll('a[href="/books/b1"]')).toHaveLength(1)
    expect(container.querySelector('[data-cover]')?.className).toMatch(/bg-cover-[1-6]/)
    expect(screen.getByText('12/40')).toBeInTheDocument()
    expect(screen.getByText('38.200')).toBeInTheDocument()
    expect(screen.getByRole('img', { name: '30%' })).toBeInTheDocument()
    expect(container.querySelector('[data-provider-icon="openrouter"]')).not.toBeNull()
    expect(screen.getByText('claude-sonnet-4')).toBeInTheDocument()
    expect(screen.getByText(/1,50 \/ 5 US\$/)).toBeInTheDocument()
  })

  it('truyện thuê bao hiện token thay USD', async () => {
    await renderWithProviders(
      <BookCard book={book({ provider: 'zai-coding', model: 'glm-4.7', subscription: true, budget_usd: 0 })} />,
    )
    expect(screen.getByText('123.456 token')).toBeInTheDocument()
    expect(screen.queryByText(/US\$/)).toBeNull()
  })

  it('lỗi có nút "Xem lỗi" tới trang truyện; có lastRead thì thêm "Đọc tiếp chương N"', async () => {
    setLastRead('b1', 4)
    const { container } = await renderWithProviders(
      <BookCard book={book({ status: 'failed', last_error: 'provider từ chối' })} />,
    )
    const links = [...container.querySelectorAll('a')].map((a) => a.textContent)
    expect(links).toContain('Xem lỗi')
    expect(screen.getByRole('link', { name: 'Đọc tiếp chương 4' })).toHaveAttribute('href', '/books/b1/read/4')
    expect(container.querySelector(NESTED)).toBeNull()
  })
})

describe('quickActions', () => {
  it('hoàn thành: đọc từ đầu, hoặc đọc tiếp nếu đã đọc dở', () => {
    const b = book({ status: 'completed' })
    expect(quickActions(b, null)).toEqual([{ kind: 'read', label: 'Đọc từ đầu', chapter: 1 }])
    expect(quickActions(b, 7)).toEqual([{ kind: 'read', label: 'Đọc tiếp chương 7', chapter: 7 }])
  })

  it('chờ duyệt chưa có chương: chỉ có "Viết tiếp"', () => {
    expect(quickActions(book({ status: 'waiting_review', snapshot: undefined }), null)).toEqual([
      { kind: 'next', label: 'Viết tiếp' },
    ])
  })

  it('lastRead vượt số chương đã xong thì bỏ qua', () => {
    expect(quickActions(book({ status: 'running' }), 99)).toEqual([])
    expect(quickActions(book({ status: 'running' }), 3)).toEqual([
      { kind: 'read', label: 'Đọc tiếp chương 3', chapter: 3 },
    ])
  })

  it('shortModel bỏ tiền tố hãng', () => {
    expect(shortModel('anthropic/claude-sonnet-4')).toBe('claude-sonnet-4')
    expect(shortModel('glm-4.7')).toBe('glm-4.7')
  })
})
