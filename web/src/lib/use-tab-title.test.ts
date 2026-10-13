import { describe, expect, it } from 'vitest'
import { formatTabTitle } from './use-tab-title'

describe('formatTabTitle', () => {
  it('có tên trang, không có truyện cần xử lý', () => {
    expect(formatTabTitle('Thư viện', 0)).toBe('Thư viện · AI Novel')
  })

  it('thêm tiền tố (n) khi có truyện cần xử lý', () => {
    expect(formatTabTitle('Thư viện', 3)).toBe('(3) Thư viện · AI Novel')
  })

  it('không có tên trang thì chỉ còn tên app', () => {
    expect(formatTabTitle(undefined, 0)).toBe('AI Novel')
    expect(formatTabTitle(undefined, 1)).toBe('(1) AI Novel')
  })
})
