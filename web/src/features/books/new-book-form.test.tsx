import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { APIKey, Book, ProviderInfo } from '@/api/types.gen'
import { jsonResponse, renderWithProviders } from '@/test/render'
import { NewBookForm } from './new-book-form'

const fetchMock = vi.fn<typeof fetch>()

const model = (id: string, name: string) => ({
  id,
  name,
  context_window: 100000,
  input_cost_per_1m: 1,
  output_cost_per_1m: 4,
})
const providers: ProviderInfo[] = [
  { name: 'openrouter', label: 'OpenRouter', models: [model('or-1', 'Model OR Một'), model('or-2', 'Model OR Hai')] },
  { name: 'gemini', label: 'Gemini', models: [model('gm-1', 'Model Gemini')] },
]
const keys: APIKey[] = [
  { id: 'k1', provider: 'openrouter', label: 'Chính', last4: '7f2a', created_at: '2026-10-12T00:00:00Z' },
  { id: 'k2', provider: 'gemini', label: 'Cá nhân', last4: '91cd', created_at: '2026-10-13T00:00:00Z' },
]

function serve(keyList: APIKey[], onCreate?: (body: unknown) => void) {
  fetchMock.mockImplementation((input, init) => {
    const url = String(input)
    if (url === '/api/keys') return Promise.resolve(jsonResponse(keyList))
    if (url === '/api/providers') return Promise.resolve(jsonResponse(providers))
    if (url === '/api/books' && init?.method === 'POST') {
      onCreate?.(JSON.parse(String(init.body)))
      return Promise.resolve(jsonResponse({ id: 'b1' } as Book, 201))
    }
    return Promise.resolve(jsonResponse({ error: { code: 'nf', message: 'không có' } }, 404))
  })
}

beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock)
})
afterEach(() => {
  fetchMock.mockReset()
  vi.unstubAllGlobals()
})

describe('NewBookForm', () => {
  it('chưa có key thì dẫn sang cài đặt key', async () => {
    serve([])
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    const link = await screen.findByRole('link', { name: 'Thêm API key' })
    expect(link).toHaveAttribute('href', '/settings/keys')
  })

  it('lọc model theo provider của key đã chọn', async () => {
    serve(keys)
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    const modelSelect = await screen.findByLabelText('Model')
    expect(Array.from((modelSelect as HTMLSelectElement).options).map((o) => o.text)).toEqual([
      'Model OR Một',
      'Model OR Hai',
    ])
    await user.selectOptions(screen.getByLabelText('API key'), 'k2')
    expect(Array.from((screen.getByLabelText('Model') as HTMLSelectElement).options).map((o) => o.text)).toEqual([
      'Model Gemini',
    ])
  })

  it('không gửi khi thiếu ý tưởng và báo lỗi', async () => {
    serve(keys)
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.click(await screen.findByRole('button', { name: /Bắt đầu viết/ }))
    expect(await screen.findByText('Hãy nhập ý tưởng cho truyện')).toBeInTheDocument()
    expect(fetchMock.mock.calls.some(([, init]) => init?.method === 'POST')).toBe(false)
  })

  it('báo lỗi ngân sách không hợp lệ', async () => {
    serve(keys)
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.type(await screen.findByLabelText('Bạn muốn viết truyện gì?'), 'Một câu chuyện')
    const budget = screen.getByLabelText('Ngân sách tối đa cho truyện này')
    await user.clear(budget)
    await user.type(budget, '0')
    await user.click(screen.getByRole('button', { name: /Bắt đầu viết/ }))
    expect(await screen.findByText(/Ngân sách phải lớn hơn 0/)).toBeInTheDocument()
  })

  it('gửi đúng dữ liệu (mặc định duyệt từng chương, ngân sách 5) rồi gọi onCreated', async () => {
    let sent: unknown
    serve(keys, (b) => (sent = b))
    const onCreated = vi.fn()
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={onCreated} />)
    await user.type(await screen.findByLabelText('Bạn muốn viết truyện gì?'), '  Kiếm khách mù  ')
    await user.selectOptions(screen.getByLabelText('Model'), 'or-2')
    await user.click(screen.getByRole('button', { name: /Bắt đầu viết/ }))
    await waitFor(() => expect(onCreated).toHaveBeenCalled())
    expect(sent).toEqual({
      prompt: 'Kiếm khách mù',
      api_key_id: 'k1',
      model: 'or-2',
      budget_usd: 5,
      advance_mode: 'review',
    })
  })

  it('tắt công tắc duyệt thì advance_mode là auto', async () => {
    let sent: { advance_mode?: string } = {}
    serve(keys, (b) => (sent = b as typeof sent))
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.type(await screen.findByLabelText('Bạn muốn viết truyện gì?'), 'x')
    await user.click(screen.getByRole('switch'))
    await user.click(screen.getByRole('button', { name: /Bắt đầu viết/ }))
    await waitFor(() => expect(sent.advance_mode).toBe('auto'))
  })

  it('hiện thông báo lỗi của server', async () => {
    fetchMock.mockImplementation((input, init) => {
      const url = String(input)
      if (url === '/api/keys') return Promise.resolve(jsonResponse(keys))
      if (url === '/api/providers') return Promise.resolve(jsonResponse(providers))
      if (init?.method === 'POST') {
        return Promise.resolve(jsonResponse({ error: { code: 'invalid_budget', message: 'Ngân sách không hợp lệ' } }, 400))
      }
      return Promise.resolve(jsonResponse({}, 404))
    })
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.type(await screen.findByLabelText('Bạn muốn viết truyện gì?'), 'x')
    await user.click(screen.getByRole('button', { name: /Bắt đầu viết/ }))
    expect(await screen.findByText('Ngân sách không hợp lệ')).toBeInTheDocument()
  })
})
