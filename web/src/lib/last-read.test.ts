import { afterEach, describe, expect, it, vi } from 'vitest'
import { getLastRead, lastReadKey, setLastRead } from './last-read'

afterEach(() => {
  vi.restoreAllMocks()
  localStorage.clear()
})

describe('last-read', () => {
  it('ghi rồi đọc lại theo từng truyện', () => {
    expect(getLastRead('b1')).toBeNull()
    setLastRead('b1', 7)
    setLastRead('b2', 2)
    expect(getLastRead('b1')).toBe(7)
    expect(getLastRead('b2')).toBe(2)
    expect(localStorage.getItem(lastReadKey('b1'))).toBe('7')
    expect(lastReadKey('b1')).toBe('ainovel:last-read:b1')
  })

  it('bỏ qua số không hợp lệ khi ghi', () => {
    setLastRead('b1', 0)
    setLastRead('b1', -3)
    setLastRead('b1', 1.5)
    expect(getLastRead('b1')).toBeNull()
  })

  it('giá trị hỏng trong storage trả null', () => {
    for (const bad of ['abc', '0', '-1', '2.5', '']) {
      localStorage.setItem(lastReadKey('b1'), bad)
      expect(getLastRead('b1')).toBeNull()
    }
  })

  it('storage ném lỗi thì không vỡ', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('SecurityError')
    })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('QuotaExceededError')
    })
    expect(() => setLastRead('b1', 3)).not.toThrow()
    expect(getLastRead('b1')).toBeNull()
  })
})
