import { describe, expect, it } from 'vitest'
import { routeKind, routeTitle } from './app-shell'

describe('routeKind', () => {
  it('phân biệt reader, trang truyện và trang khác', () => {
    expect(routeKind('/books/abc/read/3')).toBe('reader')
    expect(routeKind('/books/abc')).toBe('book')
    expect(routeKind('/books/new')).toBe('other')
    expect(routeKind('/')).toBe('other')
  })
})

describe('routeTitle', () => {
  it.each([
    ['/', 'Thư viện'],
    ['/books/new', 'Truyện mới'],
    ['/books/new/', 'Truyện mới'],
    ['/settings/keys', 'Cài đặt'],
    ['/settings/account', 'Tài khoản'],
    ['/books/abc', 'Truyện'],
    ['/books/abc/read/2', 'Đọc truyện'],
  ])('%s → %s', (path, title) => {
    expect(routeTitle(path)).toBe(title)
  })

  it('route lạ thì không có tên trang', () => {
    expect(routeTitle('/khong-co')).toBeUndefined()
  })
})
