import { useEffect, useRef } from 'react'

export type ReaderShortcut = 'prev' | 'next' | 'exit'

const KEYS: Record<string, ReaderShortcut> = {
  ArrowLeft: 'prev',
  ArrowRight: 'next',
  Escape: 'exit',
}

/** Focus đang ở chỗ gõ chữ hoặc control tự dùng phím mũi tên: không bắt phím. */
function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  if (target.isContentEditable) return true
  return target.closest('input, textarea, select, [contenteditable=""], [contenteditable="true"], [role="dialog"]') !== null
}

/**
 * Phím tắt của reader cho một sự kiện, `null` nếu phải bỏ qua:
 * có modifier, đang gõ trong input/textarea/contenteditable, sự kiện đã được xử lý hoặc đang giữ phím.
 */
export function readerShortcut(e: KeyboardEvent): ReaderShortcut | null {
  if (e.defaultPrevented || e.repeat || e.altKey || e.ctrlKey || e.metaKey || e.shiftKey) return null
  const action = KEYS[e.key]
  if (!action || isTypingTarget(e.target)) return null
  return action
}

export interface ReaderShortcutHandlers {
  onPrev?: () => void
  onNext?: () => void
  onExit: () => void
  /** Tắt toàn bộ phím tắt (vd. khi bảng cài đặt đọc đang mở). */
  disabled?: boolean
}

/** ← / → chuyển chương trước/sau, Esc về trang truyện. */
export function useReaderShortcuts(handlers: ReaderShortcutHandlers): void {
  // Giữ handler mới nhất trong ref để không gắn lại listener mỗi lần render.
  const ref = useRef(handlers)
  useEffect(() => {
    ref.current = handlers
  })

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const h = ref.current
      if (h.disabled) return
      const action = readerShortcut(e)
      const run = action === 'prev' ? h.onPrev : action === 'next' ? h.onNext : action === 'exit' ? h.onExit : undefined
      if (!run) return
      e.preventDefault()
      run()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
}
