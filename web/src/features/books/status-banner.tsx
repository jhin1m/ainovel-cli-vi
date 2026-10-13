import { Link } from '@tanstack/react-router'
import { useState, type ReactNode } from 'react'
import type { Book } from '@/api/types.gen'
import { Button, buttonClass } from '@/components/ui/button'
import { formatUSD } from '@/lib/format'
import { msg } from '@/lib/messages'
import { ChangeKeyPanel } from './change-key-panel'

const MAX_ERROR_RUNES = 240

export function truncate(text: string, max = MAX_ERROR_RUNES): string {
  const runes = [...text.trim()]
  return runes.length > max ? runes.slice(0, max).join('') + '…' : text.trim()
}

interface Props {
  book: Book
  busy?: boolean
  onNext: () => void
  onResume: () => void
  onRaiseBudget: () => void
  /** Thông báo sau khi đổi key xong (banner tự biến mất nên trang truyện hiện thay). */
  onNotice?: (text: string, status: string) => void
  /** Đổi key xong nhưng resume lỗi. */
  onNoticeError?: (text: string, status: string) => void
}

interface BoxProps {
  tone: string
  title: string
  children?: ReactNode
  actions?: ReactNode
  /**
   * Vùng live (`role="status"`) để báo khi banner xuất hiện. Tắt cho banner có ô nhập/chọn:
   * vùng live đọc lại toàn bộ nội dung mỗi lần thay đổi.
   */
  live?: boolean
}

function Box({ tone, title, children, actions, live = true }: BoxProps) {
  return (
    <section role={live ? 'status' : undefined} aria-label={title} className={`flex flex-col gap-3 rounded-card p-4 ${tone}`}>
      <div className="flex flex-col gap-1">
        <h2 className="m-0 font-serif text-lg leading-snug font-semibold">{title}</h2>
        {children && <div className="text-sm leading-relaxed break-words">{children}</div>}
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </section>
  )
}

/** Trạng thái không cần key nữa: banner "key đã bị xoá" không hiện. */
const NO_KEY_NEEDED = new Set(['completed', 'deleting'])

/**
 * Provider từ chối hẳn (sai gói/sai provider, vd. 1113). Provider của truyện cố định nên đổi
 * key chỉ giúp khi user có key khác cùng provider đúng gói; hướng chính là tạo truyện mới.
 * Phân loại bằng `stop_reason`, không dò chuỗi `last_error`.
 */
function RejectedBanner({ book, onNotice, onNoticeError }: Pick<Props, 'book' | 'onNotice' | 'onNoticeError'>) {
  const [picking, setPicking] = useState(false)
  const t = msg.book.banner
  return (
    <Box
      tone="bg-st-error-bg text-st-error"
      title={t.rejectedTitle}
      live={false}
      actions={
        <div className="flex w-full flex-col gap-3">
          <div className="flex flex-wrap gap-2">
            <Link
              to="/books/new"
              search={{ from: book.id }}
              className={buttonClass('primary', 'no-underline text-on-accent hover:text-on-accent')}
            >
              {t.newBookOther}
            </Link>
            <Button variant="secondary" aria-expanded={picking} onClick={() => setPicking((v) => !v)}>
              {picking ? t.hidePicker : t.pickSameProvider}
            </Button>
          </div>
          {picking && <ChangeKeyPanel book={book} onDone={onNotice} onResumeError={onNoticeError} />}
        </div>
      }
    >
      {book.last_error.trim() ? truncate(book.last_error, 300) : msg.book.banner.failedNoDetail}
    </Box>
  )
}

/** Banner theo trạng thái truyện; trạng thái không cần hành động thì không hiện. */
export function StatusBanner({ book, busy = false, onNext, onResume, onRaiseBudget, onNotice, onNoticeError }: Props) {
  const s = book.snapshot
  const done = s?.completed_count ?? 0

  if (book.status === 'failed' && book.stop_reason === 'provider_rejected') {
    return <RejectedBanner book={book} onNotice={onNotice} onNoticeError={onNoticeError} />
  }
  if (book.api_key_id === '' && !NO_KEY_NEEDED.has(book.status)) {
    return (
      <Box
        tone="bg-st-budget-bg text-st-budget"
        title={msg.book.banner.keyMissingTitle}
        live={false}
        actions={
          <div className="w-full">
            <ChangeKeyPanel book={book} onDone={onNotice} onResumeError={onNoticeError} />
          </div>
        }
      >
        {msg.book.banner.keyMissingBody}
      </Box>
    )
  }

  switch (book.status) {
    case 'waiting_review':
      return (
        <Box
          tone="bg-st-review-bg text-st-review"
          title={msg.book.banner.reviewTitle(done)}
          actions={
            <>
              {done > 0 && (
                <Link
                  to="/books/$id/read/$n"
                  params={{ id: book.id, n: String(done) }}
                  className={buttonClass('secondary', 'no-underline')}
                >
                  {msg.book.banner.read(done)}
                </Link>
              )}
              <Button disabled={busy} onClick={onNext}>
                {msg.book.banner.next}
              </Button>
            </>
          }
        >
          {msg.book.banner.reviewBody}
        </Box>
      )
    case 'budget_exhausted':
      return (
        <Box
          tone="bg-st-budget-bg text-st-budget"
          title={msg.book.banner.budgetTitle(formatUSD(book.budget_usd))}
          actions={<Button onClick={onRaiseBudget}>{msg.book.banner.raiseBudget}</Button>}
        >
          {msg.book.banner.budgetBody}
        </Box>
      )
    case 'paused': {
      const resume = (
        <Button disabled={busy} onClick={onResume}>
          {msg.book.resume}
        </Button>
      )
      if (s?.has_advance_hold) {
        return (
          <Box tone="bg-st-idle-bg text-st-idle" title={msg.book.banner.holdTitle} actions={resume}>
            {s.advance_hold_reason}
          </Box>
        )
      }
      return (
        <Box tone="bg-st-idle-bg text-st-idle" title={msg.book.banner.pausedTitle} actions={resume}>
          {msg.book.banner.pausedBody}
        </Box>
      )
    }
    case 'failed':
      return (
        <Box
          tone="bg-st-error-bg text-st-error"
          title={msg.book.banner.failedTitle}
          actions={
            <Button disabled={busy} onClick={onResume}>
              {msg.retry}
            </Button>
          }
        >
          {book.last_error.trim() ? truncate(book.last_error) : msg.book.banner.failedNoDetail}
        </Box>
      )
    case 'interrupted':
      return (
        <Box
          tone="bg-st-error-bg text-st-error"
          title={msg.book.banner.interruptedTitle}
          actions={
            <Button disabled={busy} onClick={onResume}>
              {msg.book.resume}
            </Button>
          }
        >
          {msg.book.banner.interruptedBody}
        </Box>
      )
    case 'queued':
      return (
        <Box tone="bg-st-idle-bg text-st-idle" title={msg.book.banner.queuedTitle}>
          {msg.book.banner.queuedBody}
        </Box>
      )
    case 'completed':
      return (
        <Box tone="bg-st-idle-bg text-fg" title={msg.book.banner.completedTitle}>
          {msg.book.banner.completedBody}
        </Box>
      )
    default:
      return null
  }
}
