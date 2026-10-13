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

export const PALETTES: Record<ReaderTheme, { bg: string; fg: string; panel: string }> = {
  light: { bg: 'var(--reader-light-bg)', fg: 'var(--reader-light-fg)', panel: '#f4f5f3' },
  sepia: { bg: 'var(--reader-sepia-bg)', fg: 'var(--reader-sepia-fg)', panel: '#eadfc9' },
  dark: { bg: 'var(--reader-dark-bg)', fg: 'var(--reader-dark-fg)', panel: '#1e2126' },
}

export function sanitizeSettings(raw: unknown): ReaderSettings {
  const o = typeof raw === 'object' && raw !== null ? (raw as Record<string, unknown>) : {}
  const size = typeof o.size === 'number' && Number.isFinite(o.size) ? Math.round(o.size) : DEFAULT_SETTINGS.size
  return {
    size: Math.min(MAX_SIZE, Math.max(MIN_SIZE, size)),
    font: o.font === 'sans' || o.font === 'serif' ? o.font : DEFAULT_SETTINGS.font,
    theme: o.theme === 'light' || o.theme === 'sepia' || o.theme === 'dark' ? o.theme : DEFAULT_SETTINGS.theme,
  }
}

export function loadSettings(): ReaderSettings {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    return sanitizeSettings(raw ? JSON.parse(raw) : null)
  } catch {
    return DEFAULT_SETTINGS
  }
}

export function saveSettings(s: ReaderSettings): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(s))
  } catch {
    // Chế độ riêng tư hoặc hết dung lượng: bỏ qua, cài đặt chỉ có hiệu lực trong phiên.
  }
}
