import { Link } from '@tanstack/react-router'
import { ArrowLeft, Download } from 'lucide-react'
import { useRef, useState, type ReactNode } from 'react'
import { download, errorMessage } from '@/api/client'
import type { Book } from '@/api/types.gen'
import { StatusBadge } from '@/components/status-badge'
import { buttonClass } from '@/components/ui/button'
import { FormError } from '@/components/ui/field'
import { getLastRead } from '@/lib/last-read'
import { msg } from '@/lib/messages'
import { bookTitle } from './book-card'

type ExportFormat = 'epub' | 'txt'

function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 10_000)
}

export function ExportMenu({ bookId, disabled }: { bookId: string; disabled: boolean }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const menuRef = useRef<HTMLDetailsElement>(null)

  if (disabled) {
    return (
      <span aria-disabled="true" title={msg.book.exportEmpty} className={buttonClass('secondary', 'opacity-60')}>
        <Download aria-hidden size={16} />
        {msg.book.export}
      </span>
    )
  }

  const run = async (format: ExportFormat) => {
    menuRef.current?.removeAttribute('open')
    setBusy(true)
    setError('')
    try {
      const { blob, filename } = await download(
        `/api/books/${encodeURIComponent(bookId)}/export?format=${format}`,
      )
      saveBlob(blob, filename || `truyen.${format}`)
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  const item =
    'flex min-h-[44px] w-full cursor-pointer items-center border-0 bg-transparent px-4 text-left text-base text-fg hover:bg-track disabled:opacity-60'
  return (
    <div className="flex flex-col gap-2">
      <details ref={menuRef} className="relative">
        <summary className={`${buttonClass('secondary')} list-none`} aria-busy={busy}>
          <Download aria-hidden size={16} />
          {busy ? msg.book.exporting : msg.book.export}
        </summary>
        <div className="absolute right-0 z-20 mt-1 w-48 overflow-hidden rounded-control border border-line bg-surface shadow-lg">
          <button type="button" className={item} disabled={busy} onClick={() => void run('epub')}>
            {msg.book.exportEpub}
          </button>
          <button type="button" className={item} disabled={busy} onClick={() => void run('txt')}>
            {msg.book.exportTxt}
          </button>
        </div>
      </details>
      {error && <FormError>{error}</FormError>}
    </div>
  )
}

interface Props {
  book: Book
  /** Nút điều khiển (dừng/tiếp tục, xuất file). */
  controls?: ReactNode
  /** Chỗ cho menu "⋯" ở góc phải tiêu đề. */
  actions?: ReactNode
}

/** Tiêu đề trang truyện: nút về, tên truyện, trạng thái, nút điều khiển và slot `actions`. */
export function BookHeader({ book, controls, actions }: Props) {
  const s = book.snapshot
  // Chỉ nhắc chương đã đọc nếu chương đó còn tồn tại (cùng quy tắc với thẻ thư viện).
  const lastRead = getLastRead(book.id)
  const resume = lastRead !== null && lastRead <= (s?.completed_count ?? 0) ? lastRead : null
  return (
    <header
      className={`pt-safe grid grid-cols-[auto_minmax(0,1fr)_auto] items-start gap-x-2 gap-y-3 md:gap-x-4 ${
        actions ? 'md:grid-cols-[auto_minmax(0,1fr)_auto_auto]' : ''
      }`}
    >
      <Link to="/" aria-label={msg.book.back} className="-ml-2 flex size-11 items-center justify-center text-fg">
        <ArrowLeft aria-hidden size={22} />
      </Link>
      <div className="flex min-w-0 flex-col gap-1.5 pt-1">
        <h1 className="m-0 font-serif text-2xl leading-snug font-semibold break-words">{bookTitle(book)}</h1>
        <div className="flex flex-wrap items-center gap-2">
          <StatusBadge status={book.status} />
          {s && s.current_chapter > 0 && (
            <span className="text-sm text-muted">
              {msg.book.chapterOf(
                s.total_chapters > 0 ? Math.min(s.current_chapter, s.total_chapters) : s.current_chapter,
                s.total_chapters,
              )}
            </span>
          )}
          {resume && (
            <Link
              to="/books/$id/read/$n"
              params={{ id: book.id, n: String(resume) }}
              className="inline-flex min-h-11 items-center text-sm font-medium"
            >
              {msg.library.actions.continueReading(resume)}
            </Link>
          )}
        </div>
      </div>
      {/* Mobile: nút điều khiển xuống hàng dưới; từ md nằm bên phải tiêu đề, trước slot `actions`. */}
      <div className="col-span-full row-start-2 flex flex-wrap gap-2 md:col-span-1 md:col-start-3 md:row-start-1 md:justify-end md:pt-1">
        {controls}
      </div>
      {actions && <div className="col-start-3 row-start-1 md:col-start-4 md:pt-1">{actions}</div>}
    </header>
  )
}
