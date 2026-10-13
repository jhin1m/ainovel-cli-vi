import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, api, request, setUnauthorizedHandler } from './client'

const fetchMock = vi.fn<typeof fetch>()

function res(body: BodyInit | null, status: number): Response {
  return new Response(body, { status })
}

beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock)
})
afterEach(() => {
  fetchMock.mockReset()
  vi.unstubAllGlobals()
})

describe('api client', () => {
  it('gửi JSON same-origin và trả dữ liệu', async () => {
    fetchMock.mockResolvedValue(res('{"ok":true}', 200))
    const out = await api.post<{ ok: boolean }>('/api/x', { a: 1 })
    expect(out).toEqual({ ok: true })
    const [path, init] = fetchMock.mock.calls[0]!
    expect(path).toBe('/api/x')
    expect(init?.credentials).toBe('same-origin')
    expect(new Headers(init?.headers).get('Content-Type')).toBe('application/json')
    expect(init?.body).toBe('{"a":1}')
  })

  it('đặt Content-Type cả với GET và chấp nhận 204', async () => {
    fetchMock.mockResolvedValue(res(null, 204))
    await expect(api.del('/api/keys/1')).resolves.toBeUndefined()
    expect(new Headers(fetchMock.mock.calls[0]![1]?.headers).get('Content-Type')).toBe('application/json')
  })

  it('ném ApiError với code/message từ server', async () => {
    fetchMock.mockResolvedValue(
      res('{"error":{"code":"wrong_password","message":"Mật khẩu hiện tại không đúng"}}', 403),
    )
    const err = await api.get('/api/x').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 403, code: 'wrong_password', message: 'Mật khẩu hiện tại không đúng' })
  })

  it('429 luôn báo thao tác quá nhanh', async () => {
    fetchMock.mockResolvedValue(res('{"error":{"code":"x","message":"khác"}}', 429))
    await expect(api.get('/api/x')).rejects.toMatchObject({
      status: 429,
      message: 'Thao tác quá nhanh, thử lại sau',
    })
  })

  it('thân không phải JSON thì dùng thông báo mặc định', async () => {
    fetchMock.mockResolvedValue(res('<html>Bad Gateway</html>', 502))
    await expect(api.get('/api/x')).rejects.toMatchObject({ status: 502, code: 'error' })
  })

  it('lỗi mạng thành ApiError status 0', async () => {
    fetchMock.mockRejectedValue(new TypeError('Failed to fetch'))
    await expect(api.get('/api/x')).rejects.toMatchObject({ status: 0, code: 'network' })
  })

  it('401 gọi handler chuyển về /login, trừ endpoint auth và /api/me', async () => {
    const onUnauthorized = vi.fn()
    setUnauthorizedHandler(onUnauthorized)
    fetchMock.mockImplementation(() => Promise.resolve(res('{"error":{"code":"unauthorized","message":"Bạn cần đăng nhập"}}', 401)))

    await expect(request('GET', '/api/books')).rejects.toBeInstanceOf(ApiError)
    expect(onUnauthorized).toHaveBeenCalledTimes(1)

    await expect(request('POST', '/api/auth/login', {})).rejects.toBeInstanceOf(ApiError)
    await expect(request('GET', '/api/me')).rejects.toBeInstanceOf(ApiError)
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
  })
})
