import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { APIKey, Book, BookSnapshot } from '@/api/types.gen'
import { jsonResponse, renderWithProviders } from '@/test/render'
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
  const h = { onNext: vi.fn(), onResume: vi.fn(), onRaiseBudget: vi.fn(), onNotice: vi.fn() }
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

  describe('đổi key', () => {
    const fetchMock = vi.fn<typeof fetch>()
    afterEach(() => {
      vi.unstubAllGlobals()
      fetchMock.mockReset()
    })
    const k2: APIKey = { id: 'k2', provider: 'zai', label: 'Dự phòng', last4: 'cd34', created_at: '' }
    function serve(keys: APIKey[]) {
      vi.stubGlobal('fetch', fetchMock)
      fetchMock.mockImplementation(async (input, init) => {
        const url = String(input)
        if (url === '/api/keys') return jsonResponse(keys)
        if (url === '/api/providers') return jsonResponse([{ name: 'zai', label: 'Z.ai', subscription: false, models: [] }])
        if (url === '/api/books/b1' && init?.method === 'PATCH') {
          return jsonResponse(book({ status: 'failed', provider: 'zai', api_key_id: 'k2' }))
        }
        return jsonResponse({ error: { code: 'nf', message: 'không có' } }, 404)
      })
    }

    it('key bị xoá: banner có ô chọn key cùng provider và gửi PATCH', async () => {
      serve([k2, { ...k2, id: 'k9', provider: 'openrouter' }])
      const h = await show(book({ status: 'paused', provider: 'zai', api_key_id: '' }))
      const banner = screen.getByRole('region', { name: 'Key của truyện đã bị xoá' })
      const select = await within(banner).findByRole('combobox', { name: 'Chọn key khác' })
      expect(within(select).getAllByRole('option')).toHaveLength(1)
      await userEvent.click(within(banner).getByRole('button', { name: 'Dùng key này' }))
      await vi.waitFor(() => expect(h.onNotice).toHaveBeenCalledWith('Đã đổi key.', 'failed'))
      const patch = fetchMock.mock.calls.find(([, init]) => init?.method === 'PATCH')
      expect(JSON.parse(String(patch?.[1]?.body))).toEqual({ api_key_id: 'k2' })
    })

    it('truyện hoàn thành thì không đòi key', async () => {
      await show(book({ status: 'completed', api_key_id: '' }))
      expect(screen.queryByRole('region', { name: 'Key của truyện đã bị xoá' })).toBeNull()
    })

    it('provider từ chối: nêu nguyên nhân, dẫn tới tạo truyện mới; chọn key cùng provider là phụ', async () => {
      serve([k2])
      await show(
        book({
          status: 'failed',
          stop_reason: 'provider_rejected',
          provider: 'zai',
          api_key_id: 'k1',
          last_error: 'Z.ai từ chối yêu cầu (mã 1113): key không thuộc gói Coding Plan.',
        }),
      )
      const banner = screen.getByRole('region', { name: 'Nhà cung cấp từ chối key' })
      expect(within(banner).getByText(/mã 1113/)).toBeInTheDocument()
      expect(within(banner).getByRole('link', { name: 'Tạo truyện mới với provider khác' })).toHaveAttribute(
        'href',
        '/books/new?from=b1',
      )
      expect(within(banner).queryByRole('button', { name: 'Thử lại' })).toBeNull()
      const toggle = within(banner).getByRole('button', { name: 'Chọn key khác cùng nhà cung cấp' })
      expect(toggle).toHaveAttribute('aria-expanded', 'false')
      await userEvent.click(toggle)
      expect(await within(banner).findByRole('combobox', { name: 'Chọn key khác' })).toBeInTheDocument()
    })

    it('lỗi thường (không phải provider_rejected) vẫn là banner lỗi có Thử lại', async () => {
      await show(book({ status: 'failed', stop_reason: 'error', api_key_id: 'k1', last_error: 'mã 1113 trong chuỗi' }))
      expect(screen.getByText('Truyện gặp lỗi')).toBeInTheDocument()
      expect(screen.queryByText('Nhà cung cấp từ chối key')).toBeNull()
    })
  })

  it('truncate keeps short text', () => {
    expect(truncate('  lỗi ngắn ')).toBe('lỗi ngắn')
  })
})
