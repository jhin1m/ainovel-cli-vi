import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { APIKey, ProviderInfo, TestKeyResult } from '@/api/types.gen'
import { jsonResponse, renderWithProviders } from '@/test/render'
import { AddKeyForm } from './add-key-form'

const fetchMock = vi.fn<typeof fetch>()

const providers: ProviderInfo[] = [
  { name: 'openrouter', label: 'OpenRouter', subscription: false, models: [] },
  { name: 'zai-coding', label: 'Z.ai Coding Plan', subscription: true, models: [] },
  { name: 'zai', label: 'Z.ai', subscription: false, models: [] },
]
const savedKey: APIKey = {
  id: 'k9',
  provider: 'zai',
  label: '',
  last4: 'ab12',
  created_at: '2026-10-13T00:00:00Z',
}

interface Calls {
  create: unknown[]
  test: unknown[]
}

/** Giả lập server: `create` và `test` trả response cho trước, ghi lại thân request. */
function serve(create: () => Response, test: () => Response): Calls {
  const calls: Calls = { create: [], test: [] }
  fetchMock.mockImplementation((input, init) => {
    const url = String(input)
    if (url === '/api/providers') return Promise.resolve(jsonResponse(providers))
    if (url === '/api/keys' && init?.method === 'POST') {
      calls.create.push(JSON.parse(String(init.body)))
      return Promise.resolve(create())
    }
    if (url === '/api/keys') return Promise.resolve(jsonResponse([]))
    if (url === '/api/keys/test' && init?.method === 'POST') {
      calls.test.push(JSON.parse(String(init.body)))
      return Promise.resolve(test())
    }
    return Promise.resolve(jsonResponse({ error: { code: 'nf', message: 'không có' } }, 404))
  })
  return calls
}

const ok: TestKeyResult = { ok: true, kind: 'ok', message: 'Key hoạt động.', model: 'glm-4.7' }

async function fillAndSave(user: ReturnType<typeof userEvent.setup>, provider: string) {
  await user.click(await screen.findByRole('radio', { name: provider }))
  await user.type(screen.getByLabelText('API key'), '  sk-secret-123  ')
  await user.click(screen.getByRole('button', { name: 'Lưu key' }))
}

beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock)
})
afterEach(() => {
  fetchMock.mockReset()
  vi.unstubAllGlobals()
})

describe('AddKeyForm', () => {
  it('lưu rồi tự kiểm đúng key vừa tạo, chỉ gửi key_id', async () => {
    const calls = serve(
      () => jsonResponse(savedKey, 201),
      () => jsonResponse(ok),
    )
    const onSaved = vi.fn()
    const user = userEvent.setup()
    await renderWithProviders(<AddKeyForm onSaved={onSaved} />)
    await fillAndSave(user, 'Z.ai')

    expect(await screen.findByText('Key hoạt động với glm-4.7')).toBeInTheDocument()
    expect(calls.create).toEqual([{ provider: 'zai', label: '', api_key: 'sk-secret-123' }])
    // Không bao giờ gửi key thô tới endpoint kiểm tra.
    expect(calls.test).toEqual([{ key_id: 'k9' }])
    expect(onSaved).toHaveBeenCalledWith(savedKey)
    expect(screen.getByText('Đã lưu key Z.ai.')).toBeInTheDocument()
  })

  it('form bị đóng lúc đang lưu thì onSaved vẫn được gọi khi server trả về', async () => {
    let release: (r: Response) => void = () => {}
    const pending = new Promise<Response>((r) => {
      release = r
    })
    fetchMock.mockImplementation((input, init) => {
      const url = String(input)
      if (url === '/api/providers') return Promise.resolve(jsonResponse(providers))
      if (url === '/api/keys' && init?.method === 'POST') return pending
      if (url === '/api/keys/test') return Promise.resolve(jsonResponse(ok))
      return Promise.resolve(jsonResponse([]))
    })
    const onSaved = vi.fn()
    const user = userEvent.setup()
    const view = await renderWithProviders(<AddKeyForm onSaved={onSaved} />)
    await fillAndSave(user, 'Z.ai')
    view.unmount()
    release(jsonResponse(savedKey, 201))
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(savedKey))
  })

  it('kiểm ra plan_mismatch thì hiện message server, mã gốc và gợi ý thêm lại', async () => {
    serve(
      () => jsonResponse(savedKey, 201),
      () =>
        jsonResponse({
          ok: false,
          kind: 'plan_mismatch',
          code: '1113',
          message: 'Key không dùng được với endpoint này.',
          model: 'glm-4.7',
        } satisfies TestKeyResult),
    )
    const user = userEvent.setup()
    await renderWithProviders(<AddKeyForm onSaved={() => {}} />)
    await fillAndSave(user, 'Z.ai')

    expect(await screen.findByText('Key không dùng được với endpoint này.')).toBeInTheDocument()
    expect(screen.getByText('Mã lỗi gốc: 1113')).toBeInTheDocument()
    expect(screen.getByText('Xoá key này và thêm lại dưới đúng nhà cung cấp.')).toBeInTheDocument()
  })

  it('kiểm bị 429 thì báo thử lại sau, key vẫn được lưu và onSaved vẫn được gọi', async () => {
    serve(
      () => jsonResponse(savedKey, 201),
      () => jsonResponse({ error: { code: 'busy', message: 'bận' } }, 429),
    )
    const onSaved = vi.fn()
    const user = userEvent.setup()
    await renderWithProviders(<AddKeyForm onSaved={onSaved} />)
    await fillAndSave(user, 'Z.ai')

    expect(await screen.findByText('Đã kiểm quá nhiều lần, thử lại sau.')).toBeInTheDocument()
    expect(onSaved).toHaveBeenCalledTimes(1)
    expect(screen.getByText('Đã lưu key Z.ai.')).toBeInTheDocument()
  })

  it('409 key_limit hiện thông báo giới hạn kèm link tới trang key, không kiểm', async () => {
    const calls = serve(
      () => jsonResponse({ error: { code: 'key_limit', message: 'quá giới hạn' } }, 409),
      () => jsonResponse(ok),
    )
    const onSaved = vi.fn()
    const user = userEvent.setup()
    await renderWithProviders(<AddKeyForm onSaved={onSaved} />)
    await fillAndSave(user, 'OpenRouter')

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('Đã đạt tối đa 20 key')
    expect(screen.getByRole('link', { name: 'Quản lý key' })).toHaveAttribute('href', '/settings/keys')
    expect(calls.test).toEqual([])
    expect(onSaved).not.toHaveBeenCalled()
  })

  it('thiếu API key thì báo lỗi, gắn aria-describedby và focus ô đó', async () => {
    const calls = serve(
      () => jsonResponse(savedKey, 201),
      () => jsonResponse(ok),
    )
    const user = userEvent.setup()
    await renderWithProviders(<AddKeyForm onSaved={() => {}} />)
    await user.click(await screen.findByRole('button', { name: 'Lưu key' }))

    const input = screen.getByLabelText('API key')
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(input).toHaveAccessibleDescription('Vui lòng dán API key')
    await waitFor(() => expect(input).toHaveFocus())
    expect(calls.create).toEqual([])
  })

  it('chọn sẵn defaultProvider và có link lấy key mở tab mới', async () => {
    serve(
      () => jsonResponse(savedKey, 201),
      () => jsonResponse(ok),
    )
    await renderWithProviders(<AddKeyForm onSaved={() => {}} defaultProvider="zai-coding" />)
    expect(await screen.findByRole('radio', { checked: true })).toHaveAccessibleName(/Z\.ai Coding Plan/)
    const link = screen.getByRole('link', { name: /Lấy key Z\.ai Coding Plan/ })
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    // Link nằm ngoài thẻ radio (không lồng phần tử tương tác).
    expect(link.closest('[role="radio"]')).toBeNull()
  })

  it('nút hiện/ẩn key đổi kiểu ô nhập', async () => {
    serve(
      () => jsonResponse(savedKey, 201),
      () => jsonResponse(ok),
    )
    const user = userEvent.setup()
    await renderWithProviders(<AddKeyForm onSaved={() => {}} />)
    const input = await screen.findByLabelText('API key')
    expect(input).toHaveAttribute('type', 'password')
    await user.click(screen.getByRole('button', { name: 'Hiện key' }))
    expect(input).toHaveAttribute('type', 'text')
    expect(screen.getByRole('button', { name: 'Hiện key' })).toHaveAttribute('aria-pressed', 'true')
  })
})
