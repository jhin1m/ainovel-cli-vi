import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Book, BookSnapshot } from '@/api/types.gen'
import { createBookStreamStore } from '@/features/book-stream/stream-store'
import { setLastRead } from '@/lib/last-read'
import { jsonResponse, renderWithProviders } from '@/test/render'
import { BookHeader } from './book-header'
import { BookScreen } from './book-screen'
import { BookStatsStrip } from './book-stats-strip'
import { SteerFab } from './steer-box'
import { WritingTab } from './writing-tab'

function book(over: Partial<Book> = {}): Book {
  return {
    id: 'b1',
    prompt: 'Một truyện tiên hiệp',
    status: 'running',
    stop_reason: '',
    provider: 'zai-coding',
    model: 'glm-4.6',
    api_key_id: 'k1',
    budget_usd: 5,
    subscription: false,
    advance_mode: '',
    pending_text: '',
    last_error: '',
    snapshot: {
      book_title: 'Kiếm Đạo',
      provider: 'zai-coding',
      model_name: 'glm-4.6',
      completed_count: 12,
      total_chapters: 40,
      total_word_count: 38200,
      total_cost_usd: 1.5,
      total_input_tokens: 1234567,
      total_output_tokens: 23456,
    } as BookSnapshot,
    created_at: '2026-10-01T00:00:00Z',
    updated_at: '2026-10-01T00:00:00Z',
    ...over,
  }
}

const stats = () => screen.getByRole('region', { name: 'Số liệu truyện' })

describe('BookStatsStrip', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('hiện tiến độ, model + icon, token vào/ra, chi phí và nút ngân sách', async () => {
    const onBudget = vi.fn()
    await renderWithProviders(<BookStatsStrip book={book()} onBudget={onBudget} />)
    const r = stats()
    expect(within(r).getByText('12/40 chương')).toBeInTheDocument()
    expect(within(r).getByRole('img', { name: '30%' })).toBeInTheDocument()
    expect(within(r).getByText('zai-coding, glm-4.6')).toBeInTheDocument()
    expect(r.querySelector('[data-provider-icon="zai-coding"], [data-provider-fallback="zai-coding"]')).not.toBeNull()
    expect(within(r).getByText('1.234.567 / 23.456')).toBeInTheDocument()
    expect(within(r).getByText('1,50 / 5 US$')).toBeInTheDocument()
    await userEvent.click(within(r).getByRole('button', { name: 'Tăng ngân sách' }))
    expect(onBudget).toHaveBeenCalledOnce()
  })

  it('thuê bao: không có USD và nút ngân sách, vẫn có token', async () => {
    await renderWithProviders(<BookStatsStrip book={book({ subscription: true, budget_usd: 0 })} />)
    const r = stats()
    expect(within(r).queryByText(/US\$/)).not.toBeInTheDocument()
    expect(within(r).queryByText('Chi phí')).not.toBeInTheDocument()
    expect(within(r).queryByRole('button')).not.toBeInTheDocument()
    expect(within(r).getByText('1.234.567 / 23.456')).toBeInTheDocument()
  })

  it('chỉ nhận focus Tab khi dải tràn ngang', async () => {
    // jsdom không tính layout: giả lập bề rộng nội dung và bề rộng khung của dải.
    let content = 300
    vi.spyOn(HTMLElement.prototype, 'scrollWidth', 'get').mockImplementation(() => content)
    vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(400)
    await renderWithProviders(<BookStatsStrip book={book()} />)
    expect(stats()).not.toHaveAttribute('tabindex')

    content = 600
    window.dispatchEvent(new Event('resize'))
    await vi.waitFor(() => expect(stats()).toHaveAttribute('tabindex', '0'))
  })
})

describe('BookHeader', () => {
  it('render slot actions đúng một lần', async () => {
    await renderWithProviders(<BookHeader book={book()} actions={<button type="button">Thêm</button>} />)
    expect(screen.getAllByRole('button', { name: 'Thêm' })).toHaveLength(1)
    expect(screen.getByRole('heading', { level: 1, name: 'Kiếm Đạo' })).toBeInTheDocument()
  })

  it('có link "Đọc tiếp chương N" khi đã đọc dở, bỏ qua chương không còn tồn tại', async () => {
    await renderWithProviders(<BookHeader book={book()} />)
    expect(screen.queryByRole('link', { name: /Đọc tiếp chương/ })).toBeNull()

    setLastRead('b1', 7)
    await renderWithProviders(<BookHeader book={book()} />)
    expect(screen.getByRole('link', { name: 'Đọc tiếp chương 7' })).toHaveAttribute('href', '/books/b1/read/7')

    setLastRead('b1', 13)
    const { container } = await renderWithProviders(<BookHeader book={book({ id: 'b1' })} />)
    expect(within(container).queryByRole('link', { name: /Đọc tiếp chương/ })).toBeNull()
    localStorage.clear()
  })
})

describe('WritingTab', () => {
  const draft = (n: number) => Array.from({ length: n }, (_, i) => `Đoạn ${i + 1}.`).join('\n\n')

  it('2 cột: hiện toàn bộ bản nháp, giới hạn 72ch, không còn 42dvh', async () => {
    const store = createBookStreamStore((fn) => fn())
    store.applyStream({ delta: draft(6) })
    const { container } = await renderWithProviders(<WritingTab book={book()} store={store} columns />)
    expect(container.innerHTML).not.toContain('42dvh')
    expect(screen.getByText('Đoạn 1.')).toBeInTheDocument()
    expect(screen.getByText('Đoạn 1.').parentElement).toHaveClass('max-w-[72ch]')
    expect(screen.queryByRole('button', { name: /Xem toàn bộ/ })).not.toBeInTheDocument()
  })

  it('xếp dọc: chỉ hiện 4 đoạn cuối, có nút xem toàn bộ và thu gọn', async () => {
    const store = createBookStreamStore((fn) => fn())
    store.applyStream({ delta: draft(6) })
    await renderWithProviders(<WritingTab book={book()} store={store} columns={false} />)
    expect(screen.queryByText('Đoạn 2.')).not.toBeInTheDocument()
    expect(screen.getByText('Đoạn 3.')).toBeInTheDocument()
    expect(screen.getByText('Đoạn 6.')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Xem toàn bộ bản nháp (2 đoạn trước)' }))
    expect(screen.getByText('Đoạn 1.')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Chỉ xem đoạn mới nhất' }))
    expect(screen.queryByText('Đoạn 1.')).not.toBeInTheDocument()
  })

  it('xếp dọc: bản nháp ngắn thì không có nút', async () => {
    const store = createBookStreamStore((fn) => fn())
    store.applyStream({ delta: draft(3) })
    await renderWithProviders(<WritingTab book={book()} store={store} columns={false} />)
    expect(screen.queryByRole('button', { name: /Xem toàn bộ/ })).not.toBeInTheDocument()
  })

  it('timeline vẫn là role=log khi có hoạt động', async () => {
    const store = createBookStreamStore((fn) => fn())
    store.upsertEvent({ id: 'e1', agent: 'writer', summary: 'Viết chương 13', finished: false } as never)
    await renderWithProviders(<WritingTab book={book()} store={store} columns={false} />)
    expect(screen.getByRole('log', { name: 'Hoạt động' })).toBeInTheDocument()
  })
})

describe('SteerFab', () => {
  it('mở sheet chứa ô góp ý và đóng được', async () => {
    await renderWithProviders(<SteerFab book={book({ pending_text: 'bớt nóng nảy' })} />)
    await userEvent.click(screen.getByRole('button', { name: 'Góp ý (có góp ý đang chờ)' }))
    const dialog = screen.getByRole('dialog', { name: 'Góp ý cho AI' })
    expect(within(dialog).getByRole('textbox', { name: 'Góp ý cho AI' })).toBeInTheDocument()
    expect(within(dialog).getByText('bớt nóng nảy')).toBeInTheDocument()
    await userEvent.click(within(dialog).getByRole('button', { name: 'Đóng' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})

class FakeEventSource {
  readyState = 0
  addEventListener() {}
  removeEventListener() {}
  close() {}
}

describe('BookScreen', () => {
  const fetchMock = vi.fn<typeof fetch>()
  afterEach(() => {
    vi.unstubAllGlobals()
    fetchMock.mockReset()
  })

  async function open(b: Book) {
    vi.stubGlobal('EventSource', FakeEventSource)
    vi.stubGlobal('fetch', fetchMock)
    fetchMock.mockImplementation(async () => jsonResponse(b))
    await renderWithProviders(<BookScreen id={b.id} />)
    return screen.findByRole('heading', { level: 1, name: 'Kiếm Đạo' })
  }

  it('nút ngân sách ở dải số liệu mở hộp thoại tăng ngân sách', async () => {
    await open(book({ provider: 'openrouter', subscription: false }))
    expect(screen.getByRole('region', { name: 'Đang làm' })).toBeInTheDocument()
    await userEvent.click(within(stats()).getByRole('button', { name: 'Tăng ngân sách' }))
    expect(screen.getByRole('dialog', { name: 'Tăng ngân sách' })).toBeInTheDocument()
  })

  it('thuê bao: không có USD, không có nút ngân sách; vẫn có nút góp ý và công tắc duyệt', async () => {
    await open(book({ subscription: true, budget_usd: 0 }))
    expect(within(stats()).queryByText(/US\$/)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Tăng ngân sách' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Góp ý' })).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: 'Duyệt từng chương' })).toBeInTheDocument()
  })

  it('tab Chương vẫn có ô góp ý inline', async () => {
    await open(book())
    fetchMock.mockImplementation(async () => jsonResponse([]))
    await userEvent.click(screen.getByRole('tab', { name: /Chương/ }))
    expect(screen.getByRole('form', { name: 'Góp ý cho AI' })).toBeInTheDocument()
  })
})
