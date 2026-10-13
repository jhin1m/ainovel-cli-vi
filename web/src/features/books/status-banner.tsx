import { Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import type { Book } from '@/api/types.gen'
import { Button, buttonClass } from '@/components/ui/button'
import { formatUSD } from '@/lib/format'
import { msg } from '@/lib/messages'

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
}

function Box({ tone, title, children, actions }: { tone: string; title: string; children?: ReactNode; actions?: ReactNode }) {
  return (
    <section role="status" aria-label={title} className={`flex flex-col gap-3 rounded-card p-4 ${tone}`}>
      <div className="flex flex-col gap-1">
        <h2 className="m-0 font-serif text-lg leading-snug font-semibold">{title}</h2>
        {children && <div className="text-sm leading-relaxed break-words">{children}</div>}
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </section>
  )
}

/** Banner theo trạng thái truyện; trạng thái không cần hành động thì không hiện. */
export function StatusBanner({ book, busy = false, onNext, onResume, onRaiseBudget }: Props) {
  const s = book.snapshot
  const done = s?.completed_count ?? 0

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
