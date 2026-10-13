import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { Book, BookSnapshot } from '@/api/types.gen'
import { renderWithProviders } from '@/test/render'
import { StatusBanner, truncate } from './status-banner'

const book = (over: Partial<Book>, snap: Partial<BookSnapshot> = {}): Book =>
  ({
    id: 'b1',
    status: 'running',
    budget_usd: 5,
    last_error: '',
    snapshot: { completed_count: 3, has_advance_hold: false, advance_hold_reason: '', ...snap },
    ...over,
  }) as Book

function show(b: Book) {
  const h = { onNext: vi.fn(), onResume: vi.fn(), onRaiseBudget: vi.fn() }
  return renderWithProviders(<StatusBanner book={b} {...h} />).then(() => h)
}

describe('StatusBanner', () => {
  it('waiting_review offers next chapter', async () => {
    const h = await show(book({ status: 'waiting_review' }))
    expect(screen.getByText('Chương 3 đã xong')).toBeInTheDocument()
    expect(screen.getByText('Đọc chương 3')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Viết chương tiếp' }))
    expect(h.onNext).toHaveBeenCalled()
  })

  it('budget_exhausted opens the raise-budget flow', async () => {
    const h = await show(book({ status: 'budget_exhausted' }))
    expect(screen.getByText(/Đã dùng hết 5,00 US\$/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Tăng ngân sách' }))
    expect(h.onRaiseBudget).toHaveBeenCalled()
  })

  it('paused with advance hold shows the hold reason', async () => {
    await show(book({ status: 'paused' }, { has_advance_hold: true, advance_hold_reason: 'Chờ biên tập duyệt' }))
    expect(screen.getByText('Chờ biên tập duyệt')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Tiếp tục' })).toBeInTheDocument()
  })

  it('plain paused shows resume', async () => {
    const h = await show(book({ status: 'paused' }))
    await userEvent.click(screen.getByRole('button', { name: 'Tiếp tục' }))
    expect(h.onResume).toHaveBeenCalled()
  })

  it('failed shows a truncated last_error and retry', async () => {
    const h = await show(book({ status: 'failed', last_error: 'x'.repeat(500) }))
    expect(screen.getByText('x'.repeat(240) + '…')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Thử lại' }))
    expect(h.onResume).toHaveBeenCalled()
  })

  it('renders nothing while running', async () => {
    await show(book({ status: 'running' }))
    expect(screen.queryByRole('status')).toBeNull()
  })

  it('truncate keeps short text', () => {
    expect(truncate('  lỗi ngắn ')).toBe('lỗi ngắn')
  })
})
