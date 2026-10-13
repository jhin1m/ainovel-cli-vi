import { renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useColumnsLayout } from './use-columns-layout'

function setup({ wide, chromeBottom, height }: { wide: boolean; chromeBottom: number; height: number }) {
  vi.stubGlobal('matchMedia', (q: string) => ({
    matches: wide,
    media: q,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
  vi.stubGlobal('innerHeight', height)
  const el = document.createElement('div')
  el.getBoundingClientRect = () => ({ bottom: chromeBottom }) as DOMRect
  return { current: el }
}

describe('useColumnsLayout', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('bật khi màn rộng và phần còn lại đủ cao', async () => {
    const ref = setup({ wide: true, chromeBottom: 400, height: 1000 })
    const { result } = renderHook(() => useColumnsLayout(ref, true))
    await waitFor(() => expect(result.current).toBe(true))
  })

  it('tắt khi phần đầu trang chiếm gần hết màn (banner trên laptop thấp)', async () => {
    const ref = setup({ wide: true, chromeBottom: 600, height: 768 })
    const { result } = renderHook(() => useColumnsLayout(ref, true))
    await new Promise((r) => requestAnimationFrame(r))
    expect(result.current).toBe(false)
  })

  it('tắt khi màn hẹp hoặc không ở tab "Đang viết"', async () => {
    const narrow = setup({ wide: false, chromeBottom: 200, height: 1000 })
    const a = renderHook(() => useColumnsLayout(narrow, true))
    await new Promise((r) => requestAnimationFrame(r))
    expect(a.result.current).toBe(false)

    const wide = setup({ wide: true, chromeBottom: 200, height: 1000 })
    const b = renderHook(() => useColumnsLayout(wide, false))
    await new Promise((r) => requestAnimationFrame(r))
    expect(b.result.current).toBe(false)
  })
})
