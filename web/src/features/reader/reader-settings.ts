export type ReaderTheme = 'light' | 'sepia' | 'dark'
export type ReaderFont = 'serif' | 'sans'

export interface ReaderSettings {
  size: number
  font: ReaderFont
  theme: ReaderTheme
}

export const MIN_SIZE = 15
export const MAX_SIZE = 24
export const DEFAULT_SETTINGS: ReaderSettings = { size: 18, font: 'serif', theme: 'light' }
export const STORAGE_KEY = 'ainovel.reader'
/**
 * Phiên bản dữ liệu lưu. Bản cũ (không có `v`) luôn ghi kèm `theme`, kể cả khi user chỉ đổi cỡ chữ,
 * nên `theme: 'light'` cũ (theme mặc định khi đó) không chắc là user tự chọn.
 */
const STORAGE_VERSION = 2

export const PALETTES: Record<ReaderTheme, { bg: string; fg: string; panel: string }> = {
  light: { bg: 'var(--reader-light-bg)', fg: 'var(--reader-light-fg)', panel: '#f4f5f3' },
  sepia: { bg: 'var(--reader-sepia-bg)', fg: 'var(--reader-sepia-fg)', panel: '#eadfc9' },
  dark: { bg: 'var(--reader-dark-bg)', fg: 'var(--reader-dark-fg)', panel: '#1e2126' },
}

/** Theme theo hệ điều hành: máy đang tối thì `dark`, còn lại `light`. */
export function systemTheme(): ReaderTheme {
  try {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  } catch {
    return DEFAULT_SETTINGS.theme
  }
}

function isTheme(v: unknown): v is ReaderTheme {
  return v === 'light' || v === 'sepia' || v === 'dark'
}

/** `fallbackTheme` dùng khi dữ liệu không có theme hợp lệ (user chưa chọn theme). */
export function sanitizeSettings(raw: unknown, fallbackTheme: ReaderTheme = DEFAULT_SETTINGS.theme): ReaderSettings {
  const o = typeof raw === 'object' && raw !== null ? (raw as Record<string, unknown>) : {}
  const size = typeof o.size === 'number' && Number.isFinite(o.size) ? Math.round(o.size) : DEFAULT_SETTINGS.size
  return {
    size: Math.min(MAX_SIZE, Math.max(MIN_SIZE, size)),
    font: o.font === 'sans' || o.font === 'serif' ? o.font : DEFAULT_SETTINGS.font,
    theme: isTheme(o.theme) ? o.theme : fallbackTheme,
  }
}

export interface StoredSettings {
  settings: ReaderSettings
  /** User đã tự chọn theme. Chưa chọn thì theme đi theo hệ điều hành và không được lưu. */
  themeChosen: boolean
}

export function loadSettings(): StoredSettings {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    const parsed: unknown = raw ? JSON.parse(raw) : null
    const o = typeof parsed === 'object' && parsed !== null ? (parsed as Record<string, unknown>) : {}
    const themeChosen = isTheme(o.theme) && (o.v === STORAGE_VERSION || o.theme !== 'light')
    const { theme, ...rest } = o
    return { settings: sanitizeSettings(themeChosen ? { ...rest, theme } : rest, systemTheme()), themeChosen }
  } catch {
    return { settings: { ...DEFAULT_SETTINGS, theme: systemTheme() }, themeChosen: false }
  }
}

/** Chỉ ghi `theme` khi user đã tự chọn, để đổi cỡ chữ không khoá theme đang theo hệ điều hành. */
export function saveSettings(s: ReaderSettings, themeChosen: boolean): void {
  try {
    const { theme, ...rest } = s
    const data = { v: STORAGE_VERSION, ...rest, ...(themeChosen ? { theme } : {}) }
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(data))
  } catch {
    // Chế độ riêng tư hoặc hết dung lượng: bỏ qua, cài đặt chỉ có hiệu lực trong phiên.
  }
}
