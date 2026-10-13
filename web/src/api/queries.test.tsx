import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { jsonResponse } from '@/test/render'
import { booksPollInterval, useCreateKey } from './queries'

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('booksPollInterval', () => {
  it('15s khi tab hiển thị, 60s khi tab ẩn', () => {
    const spy = vi.spyOn(document, 'visibilityState', 'get')
    spy.mockReturnValue('visible')
    expect(booksPollInterval()).toBe(15_000)
    spy.mockReturnValue('hidden')
    expect(booksPollInterval()).toBe(60_000)
  })
})

describe('useCreateKey', () => {
  it('không giữ key thô trong cache mutation sau khi lưu', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn<typeof fetch>((input) =>
        Promise.resolve(
          String(input) === '/api/keys'
            ? jsonResponse({ id: 'k1', provider: 'zai', label: '', last4: 'cret', created_at: '' })
            : jsonResponse([]),
        ),
      ),
    )
    const qc = new QueryClient()
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    )
    const { result, unmount } = renderHook(() => useCreateKey(), { wrapper })
    result.current.mutate({ provider: 'zai', label: '', api_key: 'sk-secret' })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    unmount()
    await waitFor(() => expect(qc.getMutationCache().getAll()).toHaveLength(0))
  })
})
