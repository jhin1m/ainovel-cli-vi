import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { StrictMode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { APIKey, Book, BookSnapshot, ProviderInfo } from '@/api/types.gen'
import { jsonResponse, renderWithProviders } from '@/test/render'
import { BookActionsMenu } from './book-actions-menu'
import { BookScreen } from './book-screen'
import { BudgetDialog } from './budget-dialog'
import { ChangeKeyPanel } from './change-key-panel'
import { DeleteBookDialog } from './delete-book-dialog'

const fetchMock = vi.fn<typeof fetch>()

class FakeEventSource {
  readyState = 0
  addEventListener() {}
  removeEventListener() {}
  close() {}
}

beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock)
  vi.stubGlobal('EventSource', FakeEventSource)
})
afterEach(() => {
  vi.unstubAllGlobals()
  fetchMock.mockReset()
})

function book(over: Partial<Book> = {}): Book {
  return {
    id: 'b1',
    prompt: 'Một truyện tiên hiệp',
    status: 'failed',
    stop_reason: '',
    provider: 'zai',
    model: 'glm-4.7',
    api_key_id: 'k1',
    budget_usd: 5,
    subscription: false,
    advance_mode: 'review',
    pending_text: '',
    last_error: '',
    snapshot: { book_title: 'Kiếm Đạo', completed_count: 3, total_cost_usd: 5 } as BookSnapshot,
    created_at: '2026-10-01T00:00:00Z',
    updated_at: '2026-10-01T00:00:00Z',
    ...over,
  }
}

const key = (id: string, provider: string, label = '', last4 = id.padStart(4, '0')): APIKey => ({
  id,
  provider,
  label,
  last4,
  created_at: '2026-10-01T00:00:00Z',
})

const providers: ProviderInfo[] = [
  { name: 'zai', label: 'Z.ai', subscription: false, models: [] },
  { name: 'openrouter', label: 'OpenRouter', subscription: false, models: [] },
]

interface Call {
  method: string
  url: string
  body?: unknown
}

/**
 * Server giả: ghi lại mọi request; trả theo bảng `routes` ("METHOD url" → [status, body]).
 * GET /api/keys và /api/providers có sẵn.
 */
function serve(keys: APIKey[], routes: Record<string, [number, unknown]> = {}) {
  const calls: Call[] = []
  fetchMock.mockImplementation(async (input, init) => {
    const url = String(input)
    const method = init?.method ?? 'GET'
    const body = typeof init?.body === 'string' ? (JSON.parse(init.body) as unknown) : undefined
    calls.push({ method, url, body })
    const hit = routes[`${method} ${url}`]
    if (hit) return hit[1] === null ? new Response(null, { status: hit[0] }) : jsonResponse(hit[1], hit[0])
    if (url === '/api/keys') return jsonResponse(keys)
    if (url === '/api/providers') return jsonResponse(providers)
    return jsonResponse({ error: { code: 'nf', message: 'không có' } }, 404)
  })
  return calls
}

const writes = (calls: Call[]) => calls.filter((c) => c.method !== 'GET').map((c) => `${c.method} ${c.url}`)

describe('ChangeKeyPanel', () => {
  it('hiện key đang dùng và chỉ liệt kê key khác cùng provider', async () => {
    serve([key('k1', 'zai', 'Cá nhân', 'ab12'), key('k2', 'zai', '', 'cd34'), key('k3', 'openrouter')])
    await renderWithProviders(<ChangeKeyPanel book={book()} />)
    expect(await screen.findByText('Key đang dùng: Z.ai · Cá nhân ••••ab12')).toBeInTheDocument()
    const select = screen.getByRole('combobox', { name: 'Chọn key khác' })
    expect(within(select).getAllByRole('option').map((o) => o.textContent)).toEqual(['Z.ai ••••cd34'])
  })

  it('"Dùng key này và viết tiếp" gửi PATCH api_key_id rồi resume', async () => {
    const calls = serve([key('k2', 'zai')], {
      'PATCH /api/books/b1': [200, book({ api_key_id: 'k2' })],
      'POST /api/books/b1/resume': [200, { book: book({ api_key_id: 'k2', status: 'queued' }), queued: true, pending: false }],
    })
    const onDone = vi.fn()
    await renderWithProviders(<ChangeKeyPanel book={book({ api_key_id: '' })} onDone={onDone} />)
    expect(await screen.findByText('Key đang dùng: không còn (key đã bị xoá)')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Dùng key này và viết tiếp' }))
    await waitFor(() => expect(onDone).toHaveBeenCalledWith('Đã đổi key. AI đang viết tiếp.', 'queued'))
    expect(writes(calls)).toEqual(['PATCH /api/books/b1', 'POST /api/books/b1/resume'])
    expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({ api_key_id: 'k2' })
  })

  it('resume lỗi thì báo rõ là key đã đổi', async () => {
    serve([key('k2', 'zai')], {
      'PATCH /api/books/b1': [200, book({ api_key_id: 'k2' })],
      'POST /api/books/b1/resume': [409, { error: { code: 'cannot_resume', message: 'Không tiếp tục được' } }],
    })
    await renderWithProviders(<ChangeKeyPanel book={book({ api_key_id: '' })} />)
    await userEvent.click(await screen.findByRole('button', { name: 'Dùng key này và viết tiếp' }))
    expect(await screen.findByText('Đã đổi key nhưng chưa viết tiếp được: Không tiếp tục được')).toBeInTheDocument()
  })

  it('trạng thái không resume được thì chỉ có "Dùng key này"', async () => {
    const calls = serve([key('k2', 'zai')], { 'PATCH /api/books/b1': [200, book({ status: 'waiting_review' })] })
    await renderWithProviders(<ChangeKeyPanel book={book({ status: 'waiting_review' })} />)
    expect(await screen.findByRole('button', { name: 'Dùng key này' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Dùng key này và viết tiếp' })).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: 'Dùng key này' }))
    expect(await screen.findByText('Đã đổi key.')).toBeInTheDocument()
    expect(writes(calls)).toEqual(['PATCH /api/books/b1'])
  })

  it('chưa có key khác cùng provider thì có link thêm key', async () => {
    serve([key('k1', 'zai'), key('k3', 'openrouter')])
    await renderWithProviders(<ChangeKeyPanel book={book()} />)
    expect(await screen.findByText('Bạn chưa có key Z.ai nào khác.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Thêm key Z.ai' })).toHaveAttribute('href', '/settings/keys')
    expect(screen.queryByRole('combobox')).toBeNull()
  })
})

describe('BudgetDialog', () => {
  const exhausted = book({ status: 'budget_exhausted' })

  it('"Lưu và viết tiếp" gửi PATCH rồi resume, đóng hộp thoại và báo kết quả', async () => {
    const calls = serve([], {
      'PATCH /api/books/b1': [200, book({ status: 'paused', budget_usd: 10 })],
      'POST /api/books/b1/resume': [200, { book: book({ status: 'queued', budget_usd: 10 }), queued: true, pending: false }],
    })
    const onClose = vi.fn()
    const onSaved = vi.fn()
    await renderWithProviders(<BudgetDialog book={exhausted} open onClose={onClose} onSaved={onSaved} />)
    await userEvent.click(screen.getByRole('button', { name: 'Lưu và viết tiếp' }))
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith('Đã tăng ngân sách. AI đang viết tiếp.', 'queued'))
    expect(onClose).toHaveBeenCalled()
    expect(writes(calls)).toEqual(['PATCH /api/books/b1', 'POST /api/books/b1/resume'])
    expect(calls[0]?.body).toEqual({ budget_usd: 10 })
  })

  it('Enter trong ô nhập chọn "Lưu và viết tiếp"', async () => {
    const calls = serve([], {
      'PATCH /api/books/b1': [200, book({ status: 'paused' })],
      'POST /api/books/b1/resume': [200, { book: book({ status: 'queued' }), queued: true, pending: false }],
    })
    await renderWithProviders(<BudgetDialog book={exhausted} open onClose={vi.fn()} />)
    await userEvent.type(screen.getByRole('textbox', { name: 'Ngân sách mới (US$)' }), '{Enter}')
    await waitFor(() => expect(writes(calls)).toEqual(['PATCH /api/books/b1', 'POST /api/books/b1/resume']))
  })

  it('"Chỉ lưu" chỉ gửi PATCH', async () => {
    const calls = serve([], { 'PATCH /api/books/b1': [200, book({ status: 'paused' })] })
    const onSaved = vi.fn()
    await renderWithProviders(<BudgetDialog book={exhausted} open onClose={vi.fn()} onSaved={onSaved} />)
    await userEvent.click(screen.getByRole('button', { name: 'Chỉ lưu' }))
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith('Đã tăng ngân sách. Bấm Tiếp tục để AI viết tiếp.', 'paused'))
    expect(writes(calls)).toEqual(['PATCH /api/books/b1'])
  })

  it('resume lỗi: báo rõ, giữ hộp thoại mở', async () => {
    serve([], {
      'PATCH /api/books/b1': [200, book({ status: 'paused' })],
      'POST /api/books/b1/resume': [409, { error: { code: 'cannot_resume', message: 'Không tiếp tục được' } }],
    })
    const onClose = vi.fn()
    await renderWithProviders(<BudgetDialog book={exhausted} open onClose={onClose} />)
    await userEvent.click(screen.getByRole('button', { name: 'Lưu và viết tiếp' }))
    expect(await screen.findByText('Đã lưu ngân sách mới nhưng chưa viết tiếp được: Không tiếp tục được')).toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('trạng thái không resume được (chờ duyệt) chỉ có nút lưu', async () => {
    serve([])
    await renderWithProviders(<BudgetDialog book={book({ status: 'waiting_review' })} open onClose={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'Lưu ngân sách' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Lưu và viết tiếp' })).toBeNull()
  })
})

describe('BookActionsMenu', () => {
  it('dùng được bằng bàn phím: mở, di chuyển, Esc đóng và trả focus', async () => {
    const onKey = vi.fn()
    await renderWithProviders(
      <BookActionsMenu
        items={[
          { id: 'key', label: 'Đổi key', onSelect: onKey },
          { id: 'delete', label: 'Xoá truyện', onSelect: vi.fn(), danger: true },
        ]}
      />,
    )
    const button = screen.getByRole('button', { name: 'Thao tác khác' })
    expect(button).toHaveAttribute('aria-haspopup', 'menu')
    expect(button).toHaveAttribute('aria-expanded', 'false')

    button.focus()
    await userEvent.keyboard('{Enter}')
    expect(button).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('menuitem', { name: 'Đổi key' })).toHaveFocus()
    await userEvent.keyboard('{ArrowDown}')
    expect(screen.getByRole('menuitem', { name: 'Xoá truyện' })).toHaveFocus()
    await userEvent.keyboard('{ArrowDown}')
    expect(screen.getByRole('menuitem', { name: 'Đổi key' })).toHaveFocus()
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('menu')).toBeNull()
    expect(button).toHaveFocus()

    await userEvent.keyboard('{ArrowUp}')
    expect(screen.getByRole('menuitem', { name: 'Xoá truyện' })).toHaveFocus()
    await userEvent.keyboard('{ArrowUp}{Enter}')
    expect(onKey).toHaveBeenCalledOnce()
    expect(screen.queryByRole('menu')).toBeNull()
    expect(button).toHaveFocus()
  })

  it('bấm ra ngoài thì đóng', async () => {
    await renderWithProviders(
      <div>
        <p>ngoài</p>
        <BookActionsMenu items={[{ id: 'a', label: 'A', onSelect: vi.fn() }]} />
      </div>,
    )
    await userEvent.click(screen.getByRole('button', { name: 'Thao tác khác' }))
    expect(screen.getByRole('menu')).toBeInTheDocument()
    await userEvent.click(screen.getByText('ngoài'))
    expect(screen.queryByRole('menu')).toBeNull()
  })
})

/** Router có trang truyện và thư viện để kiểm điều hướng sau khi xoá. */
async function renderAt(path: string, page: () => React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const root = createRootRoute()
  const home = createRoute({ getParentRoute: () => root, path: '/', component: () => <h1>Thư viện</h1> })
  const bookRoute = createRoute({ getParentRoute: () => root, path: '/books/$id', component: page })
  const router = createRouter({
    routeTree: root.addChildren([home, bookRoute]),
    history: createMemoryHistory({ initialEntries: [path] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={qc}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return { router, qc }
}

describe('DeleteBookDialog', () => {
  it('nêu tên truyện, gọi DELETE, về thư viện và bỏ cache truyện', async () => {
    const calls = serve([], { 'DELETE /api/books/b1': [202, null] })
    const b = book()
    const { router, qc } = await renderAt('/books/b1', () => <DeleteBookDialog book={b} open onClose={vi.fn()} />)
    qc.setQueryData(['book', 'b1'], b)
    const dialog = await screen.findByRole('dialog', { name: 'Xoá truyện này?' })
    expect(within(dialog).getByText(/Truyện "Kiếm Đạo" sẽ bị xoá\. Không hoàn tác được\. Mọi chương sẽ bị xoá\./)).toBeInTheDocument()
    // Nút nguy hiểm không nhận focus mặc định.
    expect(within(dialog).getByRole('button', { name: 'Xoá vĩnh viễn' })).not.toHaveFocus()
    await userEvent.click(within(dialog).getByRole('button', { name: 'Xoá vĩnh viễn' }))
    expect(await screen.findByRole('heading', { name: 'Thư viện' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/')
    expect(writes(calls)).toEqual(['DELETE /api/books/b1'])
    await waitFor(() => expect(qc.getQueryData(['book', 'b1'])).toBeUndefined())
  })

  it('lỗi thì hiện trong hộp thoại và ở lại trang', async () => {
    serve([], { 'DELETE /api/books/b1': [500, { error: { code: 'x', message: 'Lỗi máy chủ' } }] })
    const b = book()
    const { router } = await renderAt('/books/b1', () => <DeleteBookDialog book={b} open onClose={vi.fn()} />)
    await userEvent.click(await screen.findByRole('button', { name: 'Xoá vĩnh viễn' }))
    expect(await screen.findByText('Lỗi máy chủ')).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/books/b1')
  })
})

describe('BookScreen', () => {
  it('menu "⋯" → Xoá truyện mở hộp thoại xác nhận; Đổi key mở hộp thoại đổi key', async () => {
    serve([key('k1', 'zai'), key('k2', 'zai')], { 'GET /api/books/b1': [200, book({ status: 'paused' })] })
    await renderWithProviders(<BookScreen id="b1" />)
    await userEvent.click(await screen.findByRole('button', { name: 'Thao tác khác' }))
    await userEvent.click(screen.getByRole('menuitem', { name: 'Xoá truyện' }))
    expect(screen.getByRole('dialog', { name: 'Xoá truyện này?' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Huỷ' }))
    expect(screen.getByRole('button', { name: 'Thao tác khác' })).toHaveFocus()

    await userEvent.click(screen.getByRole('button', { name: 'Thao tác khác' }))
    await userEvent.click(screen.getByRole('menuitem', { name: 'Đổi key' }))
    const dialog = screen.getByRole('dialog', { name: 'Đổi API key' })
    expect(await within(dialog).findByText('Key đang dùng: Z.ai ••••00k1')).toBeInTheDocument()
  })

  it('provider từ chối: header không có "Tiếp tục" (chỉ lặp lại lỗi)', async () => {
    serve([], {
      'GET /api/books/b1': [200, book({ status: 'failed', stop_reason: 'provider_rejected', last_error: 'mã 1113' })],
    })
    await renderWithProviders(<BookScreen id="b1" />)
    expect(await screen.findByRole('region', { name: 'Nhà cung cấp từ chối key' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Tiếp tục' })).toBeNull()
  })

  it('?action=budget mở sẵn hộp thoại ngân sách và xoá action', async () => {
    serve([], { 'GET /api/books/b1': [200, book({ status: 'budget_exhausted' })] })
    const used = vi.fn()
    await renderWithProviders(<BookScreen id="b1" action="budget" onActionUsed={used} />)
    expect(await screen.findByRole('dialog', { name: 'Tăng ngân sách' })).toBeInTheDocument()
    expect(used).toHaveBeenCalledOnce()
  })

  it('?action=retry chạy lại truyện bị gián đoạn đúng một lần', async () => {
    const calls = serve([], {
      'GET /api/books/b1': [200, book({ status: 'interrupted' })],
      'POST /api/books/b1/resume': [200, { book: book({ status: 'queued' }), queued: true, pending: false }],
    })
    const used = vi.fn()
    // StrictMode chạy effect hai lần: vẫn chỉ resume một lần.
    await renderWithProviders(
      <StrictMode>
        <BookScreen id="b1" action="retry" onActionUsed={used} />
      </StrictMode>,
    )
    await waitFor(() => expect(writes(calls)).toEqual(['POST /api/books/b1/resume']))
    expect(used).toHaveBeenCalledOnce()
  })

  it('?action=retry không gọi resume khi trạng thái không cho phép', async () => {
    const calls = serve([], { 'GET /api/books/b1': [200, book({ status: 'running' })] })
    const used = vi.fn()
    await renderWithProviders(<BookScreen id="b1" action="retry" onActionUsed={used} />)
    await waitFor(() => expect(used).toHaveBeenCalledOnce())
    expect(writes(calls)).toEqual([])
  })

  it('truyện mất key: chọn key cùng provider, viết tiếp và hiện thông báo', async () => {
    const calls = serve([key('k2', 'zai')], {
      'GET /api/books/b1': [200, book({ api_key_id: '', status: 'failed' })],
      'PATCH /api/books/b1': [200, book({ api_key_id: 'k2', status: 'failed' })],
      'POST /api/books/b1/resume': [200, { book: book({ api_key_id: 'k2', status: 'queued' }), queued: true, pending: false }],
    })
    await renderWithProviders(<BookScreen id="b1" />)
    const banner = await screen.findByRole('region', { name: 'Key của truyện đã bị xoá' })
    await userEvent.click(await within(banner).findByRole('button', { name: 'Dùng key này và viết tiếp' }))
    expect(await screen.findByText('Đã đổi key. AI đang viết tiếp.')).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Key của truyện đã bị xoá' })).toBeNull()
    // Thông báo chỉ hiện một lần (ở trang), panel không lặp lại.
    expect(screen.getAllByText('Đã đổi key. AI đang viết tiếp.')).toHaveLength(1)
    expect(writes(calls)).toEqual(['PATCH /api/books/b1', 'POST /api/books/b1/resume'])
  })
})
