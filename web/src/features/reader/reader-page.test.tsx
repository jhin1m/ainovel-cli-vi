import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Book, BookSnapshot, ChapterText } from '@/api/types.gen'
import { getLastRead } from '@/lib/last-read'
import { jsonResponse, renderWithProviders } from '@/test/render'
import { ReaderPage } from './reader-page'
import { STORAGE_KEY } from './reader-settings'
import { scrollPercent } from './reading-progress'

const fetchMock = vi.fn<typeof fetch>()

function book(over: Partial<Book> = {}): Book {
  return {
    id: 'b1',
    prompt: 'Một truyện tiên hiệp',
    status: 'waiting_review',
    stop_reason: '',
    provider: 'openrouter',
    model: 'anthropic/claude-sonnet-4',
    api_key_id: 'k1',
    budget_usd: 5,
    subscription: false,
    advance_mode: '',
    pending_text: '',
    last_error: '',
    snapshot: { book_title: 'Kiếm Đạo', completed_count: 3, total_chapters: 10, in_progress_chapter: 0 } as BookSnapshot,
    created_at: '2026-10-01T00:00:00Z',
    updated_at: '2026-10-01T00:00:00Z',
    ...over,
  }
}

function chapter(over: Partial<ChapterText> = {}): ChapterText {
  return { number: 3, title: 'Gió nổi', content: 'Đoạn mở đầu.', prev: 2, next: 0, ...over }
}

/** Trả book / chapter theo URL; ghi lại các lệnh POST. */
function serve(b: Book, c: ChapterText, after?: Book) {
  fetchMock.mockImplementation(async (input, init) => {
    const url = String(input)
    if (init?.method === 'POST' && url.endsWith('/next')) return jsonResponse({ book: after ?? b })
    if (url.includes('/chapters/')) return jsonResponse(c)
    return jsonResponse(b)
  })
}

beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock)
})
afterEach(() => {
  vi.unstubAllGlobals()
  fetchMock.mockReset()
  localStorage.clear()
})

describe('ReaderPage', () => {
  it('header có tên truyện + tên chương, thanh tiến độ và nhớ chỗ đọc', async () => {
    serve(book(), chapter())
    await renderWithProviders(<ReaderPage bookId="b1" n={3} />)
    expect(await screen.findByText('Chương 3: Gió nổi')).toBeInTheDocument()
    expect(await screen.findByText('Kiếm Đạo')).toBeInTheDocument()
    const bar = screen.getByRole('progressbar', { name: 'Đã đọc' })
    expect(bar).toHaveAttribute('aria-valuenow')
    expect(getLastRead('b1')).toBe(3)
  })

  it('nút điều hướng ghi phím tắt', async () => {
    serve(book(), chapter({ number: 2, prev: 1, next: 3 }))
    await renderWithProviders(<ReaderPage bookId="b1" n={2} />)
    expect(await screen.findByRole('link', { name: /Chương trước/ })).toHaveAttribute('aria-keyshortcuts', 'ArrowLeft')
    expect(screen.getByRole('link', { name: /Chương sau/ })).toHaveAttribute('aria-keyshortcuts', 'ArrowRight')
    expect(screen.getAllByRole('link', { name: 'Về trang truyện' })[0]).toHaveAttribute('aria-keyshortcuts', 'Escape')
    // Chưa phải chương mới nhất: không có thông báo.
    expect(screen.queryByText('Đây là chương mới nhất')).toBeNull()
  })

  it('chương mới nhất của truyện chờ duyệt: viết chương tiếp ngay từ reader', async () => {
    const running = book({
      status: 'running',
      snapshot: { book_title: 'Kiếm Đạo', completed_count: 3, total_chapters: 10, in_progress_chapter: 4 } as BookSnapshot,
    })
    serve(book(), chapter(), running)
    await renderWithProviders(<ReaderPage bookId="b1" n={3} />)
    expect(await screen.findByText('Đây là chương mới nhất')).toBeInTheDocument()
    await userEvent.click(await screen.findByRole('button', { name: 'Viết chương tiếp' }))
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(expect.stringContaining('/api/books/b1/next'), expect.objectContaining({ method: 'POST' })),
    )
    expect(await screen.findByText('AI đang viết chương 4')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Viết chương tiếp' })).toBeNull()
  })

  it('truyện đang chạy: báo AI đang viết và dẫn về trang truyện', async () => {
    serve(book({ status: 'running' }), chapter())
    await renderWithProviders(<ReaderPage bookId="b1" n={3} />)
    expect(await screen.findByText('AI đang viết chương 4')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Xem AI viết' })).toHaveAttribute('href', '/books/b1')
  })

  it('truyện đã hoàn thành: chỉ có thông báo chương mới nhất', async () => {
    serve(book({ status: 'completed' }), chapter())
    await renderWithProviders(<ReaderPage bookId="b1" n={3} />)
    expect(await screen.findByText('Đây là chương mới nhất')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Viết chương tiếp' })).toBeNull()
    expect(screen.queryByText(/AI đang viết/)).toBeNull()
  })
})

describe('ReaderPage settings', () => {
  it('đổi cỡ chữ không lưu theme; bấm nút theme đang chọn sẵn thì ghim theme đó', async () => {
    serve(book(), chapter())
    await renderWithProviders(<ReaderPage bookId="b1" n={3} />)
    await userEvent.click(await screen.findByRole('button', { name: 'Cài đặt đọc' }))
    await userEvent.click(screen.getByRole('button', { name: 'Tăng cỡ chữ' }))
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '{}')).toEqual({ v: 2, size: 19, font: 'serif' })
    // jsdom không có matchMedia nên theme theo máy là "Sáng" và đang được chọn sẵn.
    const light = screen.getByRole('button', { name: 'Sáng' })
    expect(light).toHaveAttribute('aria-pressed', 'true')
    await userEvent.click(light)
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '{}')).toEqual({ v: 2, size: 19, font: 'serif', theme: 'light' })
  })

  it('chưa chọn theme thì đổi theo OS lúc đang đọc; đã chọn thì giữ nguyên', async () => {
    // matchMedia giả: đổi `dark` rồi gọi listener như trình duyệt khi OS đổi sáng/tối.
    let dark = false
    const listeners = new Set<() => void>()
    vi.stubGlobal('matchMedia', (query: string) => ({
      get matches() {
        return query.includes('dark') && dark
      },
      addEventListener: (_: string, fn: () => void) => listeners.add(fn),
      removeEventListener: (_: string, fn: () => void) => listeners.delete(fn),
    }))
    const osChange = (toDark: boolean) => {
      dark = toDark
      act(() => listeners.forEach((fn) => fn()))
    }
    serve(book(), chapter())
    await renderWithProviders(<ReaderPage bookId="b1" n={3} />)
    await userEvent.click(await screen.findByRole('button', { name: 'Cài đặt đọc' }))
    expect(screen.getByRole('button', { name: 'Sáng' })).toHaveAttribute('aria-pressed', 'true')

    osChange(true)
    expect(screen.getByRole('button', { name: 'Tối' })).toHaveAttribute('aria-pressed', 'true')
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull()

    await userEvent.click(screen.getByRole('button', { name: 'Sepia' }))
    osChange(false)
    expect(screen.getByRole('button', { name: 'Sepia' })).toHaveAttribute('aria-pressed', 'true')
  })
})

describe('scrollPercent', () => {
  it('tính phần trăm và chặn trong 0–100', () => {
    expect(scrollPercent(0, 2000, 1000)).toBe(0)
    expect(scrollPercent(500, 2000, 1000)).toBe(50)
    expect(scrollPercent(1500, 2000, 1000)).toBe(100)
    expect(scrollPercent(0, 800, 1000)).toBe(100)
  })
})
