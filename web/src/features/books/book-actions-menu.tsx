import { MoreHorizontal } from 'lucide-react'
import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import { msg } from '@/lib/messages'

export interface MenuItem {
  id: string
  label: string
  onSelect: () => void
  danger?: boolean
}

/**
 * Nút "⋯" mở menu thao tác (mẫu menu button của WAI-ARIA): Enter/Space/↓ mở và focus mục đầu,
 * ↑ mở và focus mục cuối; trong menu ↑/↓/Home/End di chuyển, Esc đóng và trả focus về nút,
 * Tab hoặc bấm ra ngoài thì đóng.
 */
export function BookActionsMenu({ items, disabled = false }: { items: MenuItem[]; disabled?: boolean }) {
  const [openState, setOpen] = useState(false)
  // Bị khoá lúc đang mở (vd. truyện chuyển sang "đang xoá") thì coi như đóng.
  const open = openState && !disabled
  // Mục được focus khi menu vừa mở (0 = đầu, -1 = cuối).
  const [initial, setInitial] = useState(0)
  const menuId = useId()
  const buttonRef = useRef<HTMLButtonElement>(null)
  const rootRef = useRef<HTMLDivElement>(null)
  const itemRefs = useRef<(HTMLButtonElement | null)[]>([])

  useEffect(() => {
    if (!open) return
    const i = initial < 0 ? items.length - 1 : initial
    itemRefs.current[i]?.focus()
    const onDown = (e: PointerEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('pointerdown', onDown)
    return () => document.removeEventListener('pointerdown', onDown)
  }, [open, initial, items.length])

  const show = (at: number) => {
    setInitial(at)
    setOpen(true)
  }

  const close = (refocus: boolean) => {
    setOpen(false)
    if (refocus) buttonRef.current?.focus()
  }

  const onButtonKey = (e: KeyboardEvent<HTMLButtonElement>) => {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      show(e.key === 'ArrowDown' ? 0 : -1)
    }
  }

  const onMenuKey = (e: KeyboardEvent<HTMLDivElement>) => {
    const list = itemRefs.current.filter((el): el is HTMLButtonElement => el !== null)
    const i = list.indexOf(document.activeElement as HTMLButtonElement)
    let next: number
    if (e.key === 'ArrowDown') next = (i + 1) % list.length
    else if (e.key === 'ArrowUp') next = (i - 1 + list.length) % list.length
    else if (e.key === 'Home') next = 0
    else if (e.key === 'End') next = list.length - 1
    else if (e.key === 'Escape') {
      e.preventDefault()
      // Không để Esc lọt ra ngoài (vd. phím tắt của trang).
      e.stopPropagation()
      close(true)
      return
    } else {
      if (e.key === 'Tab') setOpen(false)
      return
    }
    e.preventDefault()
    list[next]?.focus()
  }

  const select = (item: MenuItem) => {
    // Trả focus về nút trước khi mở hộp thoại, để hộp thoại đóng lại thì focus về đúng nút "⋯".
    close(true)
    item.onSelect()
  }

  return (
    <div ref={rootRef} className="relative">
      <button
        ref={buttonRef}
        type="button"
        aria-label={msg.book.menu.label}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        disabled={disabled}
        onClick={() => (open ? close(false) : show(0))}
        onKeyDown={onButtonKey}
        className="flex size-11 cursor-pointer items-center justify-center rounded-control border border-line bg-surface text-fg hover:bg-track disabled:cursor-not-allowed disabled:opacity-60"
      >
        <MoreHorizontal aria-hidden size={20} />
      </button>
      {open && (
        <div
          id={menuId}
          role="menu"
          aria-label={msg.book.menu.label}
          onKeyDown={onMenuKey}
          className="absolute right-0 z-20 mt-1 w-52 overflow-hidden rounded-control border border-line bg-surface py-1 shadow-lg"
        >
          {items.map((item, i) => (
            <button
              key={item.id}
              ref={(el) => {
                itemRefs.current[i] = el
              }}
              type="button"
              role="menuitem"
              tabIndex={-1}
              onClick={() => select(item)}
              className={`flex min-h-[44px] w-full cursor-pointer items-center border-0 bg-transparent px-4 text-left text-base hover:bg-track focus-visible:bg-track ${
                item.danger ? 'text-st-error' : 'text-fg'
              }`}
            >
              {item.label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
