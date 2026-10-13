import { afterEach, describe, expect, it, vi } from 'vitest'
import { DEFAULT_SETTINGS, loadSettings, saveSettings, STORAGE_KEY, systemTheme } from './reader-settings'

/** Giả lập `prefers-color-scheme` của hệ điều hành. */
function mockColorScheme(dark: boolean) {
  vi.stubGlobal(
    'matchMedia',
    vi.fn((query: string) => ({ matches: dark && query === '(prefers-color-scheme: dark)', media: query })),
  )
}

describe('reader settings', () => {
  afterEach(() => {
    localStorage.clear()
    vi.unstubAllGlobals()
  })

  it('round-trips and clamps stored values', () => {
    saveSettings({ size: 20, font: 'sans', theme: 'dark' }, true)
    expect(loadSettings()).toEqual({ settings: { size: 20, font: 'sans', theme: 'dark' }, themeChosen: true })
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ size: 999, font: 'x', theme: 'neon' }))
    expect(loadSettings()).toEqual({ settings: { ...DEFAULT_SETTINGS, size: 24 }, themeChosen: false })
  })

  it('falls back to defaults on corrupt storage', () => {
    localStorage.setItem(STORAGE_KEY, '{oops')
    expect(loadSettings()).toEqual({ settings: DEFAULT_SETTINGS, themeChosen: false })
  })

  it('máy đang tối và chưa lưu cài đặt thì reader tối', () => {
    mockColorScheme(true)
    expect(systemTheme()).toBe('dark')
    expect(loadSettings()).toEqual({ settings: { ...DEFAULT_SETTINGS, theme: 'dark' }, themeChosen: false })
  })

  it('máy sáng hoặc không có matchMedia thì theme sáng', () => {
    mockColorScheme(false)
    expect(loadSettings().settings.theme).toBe('light')
    vi.stubGlobal('matchMedia', undefined)
    expect(systemTheme()).toBe('light')
  })

  it('theme đã chọn được giữ dù hệ điều hành đổi', () => {
    mockColorScheme(true)
    saveSettings({ ...DEFAULT_SETTINGS, theme: 'sepia' }, true)
    expect(loadSettings().settings.theme).toBe('sepia')
  })

  it('chỉ đổi cỡ chữ thì không lưu theme, theme vẫn theo hệ điều hành', () => {
    mockColorScheme(true)
    saveSettings({ ...DEFAULT_SETTINGS, size: 20, theme: 'dark' }, false)
    expect(JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '{}')).toEqual({ v: 2, size: 20, font: 'serif' })
    mockColorScheme(false)
    expect(loadSettings()).toEqual({ settings: { ...DEFAULT_SETTINGS, size: 20, theme: 'light' }, themeChosen: false })
  })

  it('dữ liệu bản cũ: theme sáng mặc định không khoá theme, sepia/tối thì giữ', () => {
    mockColorScheme(true)
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ size: 20, font: 'sans', theme: 'light' }))
    expect(loadSettings()).toEqual({ settings: { size: 20, font: 'sans', theme: 'dark' }, themeChosen: false })
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ size: 20, font: 'sans', theme: 'sepia' }))
    expect(loadSettings()).toEqual({ settings: { size: 20, font: 'sans', theme: 'sepia' }, themeChosen: true })
  })

  it('bản mới: theme sáng đã chọn được giữ khi máy tối', () => {
    mockColorScheme(true)
    saveSettings({ ...DEFAULT_SETTINGS, theme: 'light' }, true)
    expect(loadSettings()).toEqual({ settings: { ...DEFAULT_SETTINGS, theme: 'light' }, themeChosen: true })
  })
})
