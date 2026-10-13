import { msg } from '@/lib/messages'
import type { ErrorResponse } from './types.gen'

export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

type UnauthorizedHandler = () => void

let onUnauthorized: UnauthorizedHandler = () => {
  const p = window.location.pathname
  if (p !== '/login' && p !== '/signup') window.location.assign('/login')
}

/** Router đăng ký cách chuyển về /login (xoá cache + điều hướng không tải lại trang). */
export function setUnauthorizedHandler(fn: UnauthorizedHandler): void {
  onUnauthorized = fn
}

// /api/auth/* tự xử lý 401 (sai mật khẩu); /api/me do route guard xử lý.
function skipsRedirect(path: string): boolean {
  return path.startsWith('/api/auth/') || path === '/api/me'
}

function fallbackMessage(status: number): string {
  if (status === 429) return msg.rateLimited
  if (status === 404) return msg.notFound
  return msg.genericError
}

async function parseError(res: Response): Promise<ApiError> {
  if (res.status === 429) return new ApiError(429, 'rate_limited', msg.rateLimited)
  try {
    const body = (await res.json()) as Partial<ErrorResponse>
    const d = body?.error
    if (d && typeof d.message === 'string' && d.message !== '') {
      return new ApiError(res.status, typeof d.code === 'string' ? d.code : 'error', d.message)
    }
  } catch {
    // Thân không phải JSON (proxy 502...), dùng thông báo mặc định.
  }
  return new ApiError(res.status, 'error', fallbackMessage(res.status))
}

export interface RequestOptions {
  signal?: AbortSignal
}

export async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  opts: RequestOptions = {},
): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      method,
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: opts.signal,
    })
  } catch (e) {
    if (e instanceof DOMException && e.name === 'AbortError') throw e
    throw new ApiError(0, 'network', msg.networkError)
  }

  if (!res.ok) {
    const err = await parseError(res)
    if (res.status === 401 && !skipsRedirect(path)) onUnauthorized()
    throw err
  }

  if (res.status === 204) return undefined as T
  const text = await res.text()
  if (text === '') return undefined as T
  try {
    return JSON.parse(text) as T
  } catch {
    throw new ApiError(res.status, 'bad_response', msg.genericError)
  }
}

export const api = {
  get: <T>(path: string, opts?: RequestOptions) => request<T>('GET', path, undefined, opts),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body ?? {}),
  patch: <T>(path: string, body: unknown) => request<T>('PATCH', path, body),
  del: <T>(path: string) => request<T>('DELETE', path),
}

export function errorMessage(e: unknown): string {
  return e instanceof ApiError ? e.message : msg.genericError
}
