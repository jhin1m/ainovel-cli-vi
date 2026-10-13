import { useEffect, useId, useRef, type ReactNode } from 'react'

type Size = 'md' | 'lg' | 'xl'

interface Props {
  open: boolean
  title: string
  onClose: () => void
  children: ReactNode
  /** Bề ngang tối đa ở màn rộng. Mặc định `md`. */
  size?: Size
}

// Lớp Tailwind viết nguyên chuỗi để bộ quét không bỏ sót.
const widths: Record<Size, string> = {
  md: 'max-w-md',
  lg: 'max-w-2xl',
  xl: 'max-w-4xl',
}

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

/** Hộp thoại modal: Esc để đóng, giữ Tab trong hộp thoại, trả focus về nút mở khi đóng. */
export function Dialog({ open, title, onClose, children, size = 'md' }: Props) {
  const titleId = useId()
  const panelRef = useRef<HTMLDivElement>(null)
  const onCloseRef = useRef(onClose)

  useEffect(() => {
    onCloseRef.current = onClose
  })

  useEffect(() => {
    if (!open) return
    const prev = document.activeElement as HTMLElement | null
    const panel = panelRef.current
    panel?.focus()
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onCloseRef.current()
        return
      }
      if (e.key !== 'Tab' || !panel) return
      // Chỉ lấy phần tử nằm trong thứ tự Tab (bỏ radio roving có tabindex=-1).
      const items = [...panel.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((el) => el.tabIndex >= 0)
      const first = items[0]
      const last = items[items.length - 1]
      if (!first || !last) {
        e.preventDefault()
        panel.focus()
      } else if (!items.includes(document.activeElement as HTMLElement)) {
        // Focus ở ngoài panel, ở chính panel, hoặc ở phần tử không nằm trong thứ tự Tab.
        e.preventDefault()
        ;(e.shiftKey ? last : first).focus()
      } else if (e.shiftKey && document.activeElement === first) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      prev?.focus?.()
    }
  }, [open])

  if (!open) return null
  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-black/50 sm:items-center sm:p-4">
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className={`pb-safe max-h-[90dvh] w-full overflow-y-auto rounded-t-card bg-surface p-5 shadow-xl sm:rounded-card ${widths[size]}`}
      >
        <h2 id={titleId} className="m-0 mb-3 font-serif text-xl font-semibold">
          {title}
        </h2>
        {children}
      </div>
    </div>
  )
}
