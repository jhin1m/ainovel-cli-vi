import { describe, expect, it, vi } from 'vitest'
import { createUpdateStore } from './pwa-update'

describe('createUpdateStore', () => {
  it('bắt đầu không có bản mới', () => {
    expect(createUpdateStore().available()).toBe(false)
  })

  it('offer bật cờ và báo cho subscriber', () => {
    const s = createUpdateStore()
    const l = vi.fn()
    s.subscribe(l)
    s.offer(() => {})
    expect(s.available()).toBe(true)
    expect(l).toHaveBeenCalledTimes(1)
  })

  it('apply gọi hàm kích hoạt đúng một lần và tắt cờ', () => {
    const s = createUpdateStore()
    const fn = vi.fn()
    s.offer(fn)
    s.apply()
    s.apply()
    expect(fn).toHaveBeenCalledTimes(1)
    expect(s.available()).toBe(false)
  })

  it('dismiss tắt cờ mà không kích hoạt', () => {
    const s = createUpdateStore()
    const fn = vi.fn()
    s.offer(fn)
    s.dismiss()
    expect(s.available()).toBe(false)
    expect(fn).not.toHaveBeenCalled()
  })

  it('unsubscribe ngừng nhận thông báo', () => {
    const s = createUpdateStore()
    const l = vi.fn()
    s.subscribe(l)()
    s.offer(() => {})
    expect(l).not.toHaveBeenCalled()
  })
})
