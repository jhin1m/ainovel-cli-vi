import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { APIKey, Book, ProviderInfo, TestKeyResult } from '@/api/types.gen'
import { jsonResponse, renderWithProviders } from '@/test/render'
import { NewBookForm } from './new-book-form'
import { UNDO_MS } from './prompt-section'

const fetchMock = vi.fn<typeof fetch>()

const model = (id: string, name: string, cost = 1, recommended = false) => ({
  id,
  name,
  context_window: 100000,
  input_cost_per_1m: cost,
  output_cost_per_1m: cost * 4,
  recommended,
})
const providers: ProviderInfo[] = [
  {
    name: 'openrouter',
    label: 'OpenRouter',
    subscription: false,
    models: [model('or-1', 'Model OR Một'), model('or-2', 'Model OR Hai')],
  },
  { name: 'gemini', label: 'Gemini', subscription: false, models: [model('gm-1', 'Model Gemini')] },
  {
    name: 'anthropic',
    label: 'Anthropic',
    subscription: false,
    models: [model('an-1', 'Rẻ', 1), model('an-2', 'Vừa', 3, true), model('an-3', 'Mạnh', 15)],
  },
  {
    name: 'zai-coding',
    label: 'Z.ai Coding Plan',
    subscription: true,
    models: [model('glm-4.7', 'GLM-5.3-Flash', 1, true), model('glm-5.1', 'GLM-5.3', 2), model('x', 'X', 3)],
  },
]
const keys: APIKey[] = [
  { id: 'k1', provider: 'openrouter', label: 'Chính', last4: '7f2a', created_at: '2026-10-12T00:00:00Z' },
  { id: 'k2', provider: 'gemini', label: 'Cá nhân', last4: '91cd', created_at: '2026-10-13T00:00:00Z' },
  { id: 'k3', provider: 'anthropic', label: '', last4: 'aaaa', created_at: '2026-10-13T00:00:00Z' },
  { id: 'k4', provider: 'zai-coding', label: '', last4: 'zzzz', created_at: '2026-10-13T00:00:00Z' },
]

const pastBook = (m: string, cost: number, done: number, subscription = false) =>
  ({ id: `${m}-${done}`, model: m, subscription, snapshot: { total_cost_usd: cost, completed_count: done } }) as Book

interface Opts {
  keyList?: APIKey[]
  books?: Book[]
  onCreate?: (body: unknown) => void
  createError?: { status: number; code: string; message: string }
  check?: TestKeyResult
}

function serve({ keyList = keys, books = [], onCreate, createError, check }: Opts = {}) {
  const list = [...keyList]
  fetchMock.mockImplementation((input, init) => {
    const url = String(input)
    const post = init?.method === 'POST'
    if (url === '/api/keys' && post) {
      const k: APIKey = { id: 'new', provider: 'openrouter', label: '', last4: '9999', created_at: '' }
      list.push(k)
      return Promise.resolve(jsonResponse(k, 201))
    }
    if (url === '/api/keys/test' && post) {
      return Promise.resolve(jsonResponse(check ?? { ok: true, kind: 'ok', model: 'Model OR Một' }))
    }
    if (url === '/api/keys') return Promise.resolve(jsonResponse([...list]))
    if (url === '/api/providers') return Promise.resolve(jsonResponse(providers))
    if (url === '/api/books' && post) {
      onCreate?.(JSON.parse(String(init.body)))
      if (createError) {
        const { status, code, message } = createError
        return Promise.resolve(jsonResponse({ error: { code, message } }, status))
      }
      return Promise.resolve(jsonResponse({ id: 'b1' } as Book, 201))
    }
    if (url === '/api/books') return Promise.resolve(jsonResponse(books))
    return Promise.resolve(jsonResponse({ error: { code: 'nf', message: 'không có' } }, 404))
  })
}

const posted = () => fetchMock.mock.calls.filter(([u, init]) => String(u) === '/api/books' && init?.method === 'POST')
const optionTexts = (label: string) =>
  Array.from((screen.getByLabelText(label) as HTMLSelectElement).options).map((o) => o.text)
const ideaBox = () => screen.findByLabelText('Bạn muốn viết truyện gì?')
const submitBtn = () => screen.getByRole('button', { name: /Bắt đầu viết/ })

beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock)
})
afterEach(() => {
  fetchMock.mockReset()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('NewBookForm', () => {
  it('chưa có key: thêm key ngay tại trang, tự chọn key đó rồi tạo được truyện', async () => {
    let sent: { api_key_id?: string } = {}
    serve({ keyList: [], onCreate: (b) => (sent = b as typeof sent) })
    const onCreated = vi.fn()
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={onCreated} />)

    expect(await screen.findByText('Thêm API key để bắt đầu')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Thêm API key' })).toBeNull()
    expect(screen.queryByLabelText('Model')).toBeNull()

    await user.type(screen.getByLabelText('API key'), 'sk-or-123')
    await user.click(screen.getByRole('button', { name: 'Lưu key' }))

    // Key mới được chọn; kết quả tự kiểm vẫn nằm trong thẻ cấu hình.
    await waitFor(() => expect(screen.getByRole('combobox', { name: 'Key đã lưu' })).toHaveValue('new'))
    expect(await screen.findByText('Key hoạt động với Model OR Một')).toBeInTheDocument()
    expect(screen.getByLabelText('Model')).toHaveValue('or-1')
    // Lưu key không được kích hoạt submit của form tạo truyện (không lồng <form>).
    expect(screen.queryByText('Hãy nhập ý tưởng cho truyện')).toBeNull()
    expect(document.querySelector('form form')).toBeNull()

    await user.type(await ideaBox(), 'Kiếm khách mù')
    await user.click(submitBtn())
    await waitFor(() => expect(onCreated).toHaveBeenCalled())
    expect(sent.api_key_id).toBe('new')
  })

  it('kết quả kiểm sai gói vẫn hiện cạnh ô chọn key và không chặn tạo truyện', async () => {
    serve({
      keyList: [],
      check: { ok: false, kind: 'plan_mismatch', code: '1113', message: 'Key không thuộc gói này', model: '' },
    })
    const onCreated = vi.fn()
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={onCreated} />)
    await user.type(await screen.findByLabelText('API key'), 'sk-x')
    await user.click(screen.getByRole('button', { name: 'Lưu key' }))
    expect(await screen.findByText('Key không thuộc gói này')).toBeInTheDocument()
    expect(screen.getByText('Mã lỗi gốc: 1113')).toBeInTheDocument()
    await user.type(await ideaBox(), 'x')
    await user.click(submitBtn())
    await waitFor(() => expect(onCreated).toHaveBeenCalled())
    // "Xong" đóng khối kết quả, hiện lại nút thêm key khác.
    await user.click(screen.getByRole('button', { name: 'Xong' }))
    expect(screen.getByRole('button', { name: 'Thêm key khác' })).toBeInTheDocument()
  })

  it('đã có key: "Thêm key khác" mở form tại chỗ, Huỷ thì đóng', async () => {
    serve()
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.click(await screen.findByRole('button', { name: 'Thêm key khác' }))
    expect(screen.getByRole('button', { name: 'Lưu key' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Huỷ' }))
    expect(screen.queryByRole('button', { name: 'Lưu key' })).toBeNull()
  })

  it('lọc model theo provider của key và chọn sẵn model đề xuất', async () => {
    serve()
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await screen.findByLabelText('Model')
    expect(optionTexts('Model')).toEqual(['Model OR Một', 'Model OR Hai'])
    await user.selectOptions(screen.getByRole('combobox', { name: 'Key đã lưu' }), 'k2')
    expect(optionTexts('Model')).toEqual(['Model Gemini'])
    await user.selectOptions(screen.getByRole('combobox', { name: 'Key đã lưu' }), 'k3')
    expect(screen.getByLabelText('Model')).toHaveValue('an-2')
    // Provider có ≥3 model có giá: có nhãn bậc giá và badge đề xuất.
    expect(optionTexts('Model')).toEqual(['Rẻ · rẻ', 'Vừa · Đề xuất · cân bằng', 'Mạnh · chất lượng'])
    expect(screen.getByText('Đề xuất', { selector: 'span' })).toBeInTheDocument()
  })

  it('provider ít hơn 3 model thì không có nhãn bậc giá', async () => {
    serve()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await screen.findByLabelText('Model')
    expect(screen.queryByText(/rẻ|cân bằng|chất lượng/)).toBeNull()
  })

  it('gói thuê bao: không có ô ngân sách, không bậc giá, gửi budget_usd 0', async () => {
    let sent: { budget_usd?: number } = {}
    serve({ onCreate: (b) => (sent = b as typeof sent) })
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Key đã lưu' }), 'k4')
    expect(screen.queryByLabelText('Ngân sách tối đa cho truyện này')).toBeNull()
    expect(screen.getByText(/Dùng quota gói Z\.ai Coding Plan, tính bằng token, không trừ USD/)).toBeInTheDocument()
    expect(optionTexts('Model')).toEqual(['GLM-5.3-Flash · Đề xuất', 'GLM-5.3', 'X'])
    expect(screen.queryByText('Chi tiết giá')).toBeNull()
    await user.type(await ideaBox(), 'x')
    await user.click(submitBtn())
    await waitFor(() => expect(sent.budget_usd).toBe(0))
  })

  it('chip gợi ý điền thẳng khi ô trống', async () => {
    serve()
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.click(await screen.findByRole('button', { name: 'Kinh dị làng quê' }))
    expect(((await ideaBox()) as HTMLTextAreaElement).value).toContain('lễ cúng cô hồn')
    expect(screen.queryByRole('button', { name: 'Hoàn tác' })).toBeNull()
  })

  it('chip gợi ý không làm mất chữ đã gõ: có hoàn tác', async () => {
    serve()
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    const box = await ideaBox()
    await user.type(box, 'Ý tưởng của tôi')
    await user.click(screen.getByRole('button', { name: 'Tu tiên quật khởi' }))
    expect((box as HTMLTextAreaElement).value).toContain('phế linh căn')
    expect(screen.getByText(/Đã thay ý tưởng bằng gợi ý "Tu tiên quật khởi"/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Hoàn tác' }))
    expect(box).toHaveValue('Ý tưởng của tôi')
    expect(box).toHaveFocus()
  })

  it('bấm lại cùng gợi ý thì vẫn giữ hoàn tác', async () => {
    serve()
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    const box = await ideaBox()
    await user.type(box, 'Chữ gốc')
    await user.dblClick(screen.getByRole('button', { name: 'Tu tiên quật khởi' }))
    await user.click(screen.getByRole('button', { name: 'Hoàn tác' }))
    expect(box).toHaveValue('Chữ gốc')
  })

  it('đổi liên tiếp hai gợi ý thì hoàn tác vẫn trả về chữ gốc', async () => {
    serve()
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    const box = await ideaBox()
    await user.type(box, 'Chữ gốc')
    await user.click(screen.getByRole('button', { name: 'Tu tiên quật khởi' }))
    await user.click(screen.getByRole('button', { name: 'Đô thị dị năng' }))
    expect(screen.getByText(/gợi ý "Đô thị dị năng"/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Hoàn tác' }))
    expect(box).toHaveValue('Chữ gốc')
  })

  it('Enter ở ô nhân vật chính không tạo truyện', async () => {
    serve()
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.type(await ideaBox(), 'x')
    await user.click(screen.getByText('Tuỳ chỉnh thêm'))
    await user.type(screen.getByLabelText('Nhân vật chính'), 'Lý Mặc{Enter}')
    expect(posted()).toHaveLength(0)
  })

  it('nút hoàn tác tự ẩn sau 5 giây', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    serve()
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.type(await ideaBox(), 'abc')
    await user.click(screen.getByRole('button', { name: 'Trinh thám phố cổ' }))
    expect(screen.getByRole('button', { name: 'Hoàn tác' })).toBeInTheDocument()
    act(() => vi.advanceTimersByTime(UNDO_MS))
    expect(screen.queryByRole('button', { name: 'Hoàn tác' })).toBeNull()
  })

  it('tuỳ chỉnh thêm ghép vào prompt, có xem trước, API không đổi', async () => {
    let sent: Record<string, unknown> = {}
    serve({ onCreate: (b) => (sent = b as typeof sent) })
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.type(await ideaBox(), 'Kiếm khách mù')
    await user.click(screen.getByText('Tuỳ chỉnh thêm'))
    await user.click(screen.getByRole('button', { name: 'Trinh thám' }))
    await user.click(screen.getByRole('button', { name: /^Ngắn/ }))
    await user.type(screen.getByLabelText('Bối cảnh'), 'Huế 1945')
    const expected = 'Kiếm khách mù\n\nThể loại: trinh thám.\nĐộ dài mong muốn: khoảng 5–10 chương.\nBối cảnh: Huế 1945.'
    expect(screen.getByTestId('prompt-preview').textContent).toBe(expected)
    // Bấm lại chip đang chọn thì bỏ chọn.
    await user.click(screen.getByRole('button', { name: 'Trinh thám' }))
    expect(screen.getByRole('button', { name: 'Trinh thám' })).toHaveAttribute('aria-pressed', 'false')
    await user.click(screen.getByRole('button', { name: 'Trinh thám' }))
    await user.click(submitBtn())
    await waitFor(() => expect(sent.prompt).toBe(expected))
    expect(Object.keys(sent).sort()).toEqual(['advance_mode', 'api_key_id', 'budget_usd', 'model', 'prompt'])
  })

  it('ước tính số chương từ số đo thật, bỏ truyện thuê bao; chưa có số đo thì nói rõ', async () => {
    serve({ books: [pastBook('or-1', 1, 4), pastBook('or-1', 0, 30, true)] })
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    // 1 US$ / 4 chương = 0,25 → 5 US$ ≈ 15–20 chương.
    expect(await screen.findByText('≈ 15–20 chương với model này')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '10 US$' }))
    expect(screen.getByLabelText('Ngân sách tối đa cho truyện này')).toHaveValue(10)
    expect(screen.getByRole('button', { name: '10 US$' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByText('≈ 30–40 chương với model này')).toBeInTheDocument()
    await user.selectOptions(screen.getByLabelText('Model'), 'or-2')
    expect(
      screen.getByText('Chưa có số đo cho model này; ước tính xuất hiện sau chương đầu tiên.'),
    ).toBeInTheDocument()
  })

  it('ngân sách chưa đủ 1 chương thì nói rõ thay vì "≈ 0 chương"', async () => {
    serve({ books: [pastBook('or-1', 10, 1)] })
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.click(await screen.findByRole('button', { name: '2 US$' }))
    expect(await screen.findByText('Ngân sách này chưa đủ cho 1 chương với model này.')).toBeInTheDocument()
  })

  it('giá token nằm trong "Chi tiết giá" bằng lời thường', async () => {
    serve()
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.click(await screen.findByText('Chi tiết giá'))
    expect(screen.getByText(/1,00 US\$ cho mỗi 1 triệu token đầu vào, 4,00 US\$ cho mỗi 1 triệu token đầu ra/)).toBeVisible()
  })

  it('không gửi khi thiếu ý tưởng, báo lỗi và focus ô ý tưởng', async () => {
    serve()
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.click(await screen.findByRole('button', { name: /Bắt đầu viết/ }))
    expect(await screen.findByText('Hãy nhập ý tưởng cho truyện')).toBeInTheDocument()
    expect(await ideaBox()).toHaveFocus()
    expect(posted()).toHaveLength(0)
  })

  it('báo lỗi ngân sách không hợp lệ và focus ô ngân sách', async () => {
    serve()
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.type(await ideaBox(), 'Một câu chuyện')
    const budget = screen.getByLabelText('Ngân sách tối đa cho truyện này')
    await user.clear(budget)
    await user.type(budget, '0')
    await user.click(submitBtn())
    expect(await screen.findByText(/Ngân sách phải lớn hơn 0/)).toBeInTheDocument()
    expect(budget).toHaveFocus()
    expect(posted()).toHaveLength(0)
  })

  it('gửi đúng dữ liệu (mặc định duyệt từng chương, ngân sách 5) rồi gọi onCreated', async () => {
    let sent: unknown
    serve({ onCreate: (b) => (sent = b) })
    const onCreated = vi.fn()
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={onCreated} />)
    await user.type(await ideaBox(), '  Kiếm khách mù  ')
    await user.selectOptions(screen.getByLabelText('Model'), 'or-2')
    await user.click(submitBtn())
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
    serve({ onCreate: (b) => (sent = b as typeof sent) })
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.type(await ideaBox(), 'x')
    await user.click(screen.getByRole('switch'))
    await user.click(submitBtn())
    await waitFor(() => expect(sent.advance_mode).toBe('auto'))
  })

  it('hiện thông báo lỗi của server', async () => {
    serve({ createError: { status: 400, code: 'invalid_budget', message: 'Ngân sách không hợp lệ' } })
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.type(await ideaBox(), 'x')
    await user.click(submitBtn())
    expect(await screen.findByText('Ngân sách không hợp lệ')).toBeInTheDocument()
  })

  it('máy chủ hết dung lượng (507) hiện thông báo chung', async () => {
    serve({ createError: { status: 507, code: 'disk_full', message: 'chi tiết nội bộ' } })
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.type(await ideaBox(), 'x')
    await user.click(submitBtn())
    const alert = await screen.findByText('Máy chủ sắp hết dung lượng, thử lại sau.')
    expect(alert).toBeInTheDocument()
    expect(within(document.body).queryByText('chi tiết nội bộ')).toBeNull()
  })

  it('chạm giới hạn số truyện: có link mở thư viện để xoá truyện cũ', async () => {
    serve({ createError: { status: 409, code: 'book_limit', message: 'Bạn đã có tối đa 20 truyện, hãy xoá bớt truyện cũ' } })
    const user = userEvent.setup()
    await renderWithProviders(<NewBookForm onCreated={() => {}} />)
    await user.type(await ideaBox(), 'x')
    await user.click(submitBtn())
    const alert = (await screen.findByText(/Bạn đã có tối đa 20 truyện/)).closest('[role="alert"]')!
    expect(within(alert as HTMLElement).getByRole('link', { name: 'Mở thư viện để xoá truyện cũ' })).toHaveAttribute('href', '/')
  })

  it('điền sẵn ý tưởng từ truyện cũ và chọn sẵn key của provider khác provider đã từ chối', async () => {
    serve()
    await renderWithProviders(
      <NewBookForm onCreated={() => {}} initialIdea="Truyện cũ bị từ chối" avoidProvider="openrouter" />,
    )
    expect(await ideaBox()).toHaveValue('Truyện cũ bị từ chối')
    expect(screen.getByLabelText('Key đã lưu')).toHaveValue('k2')
  })
})
