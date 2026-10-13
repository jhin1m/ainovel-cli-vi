import { Link } from '@tanstack/react-router'
import type { Book } from '@/api/types.gen'
import { StatusBadge } from '@/components/status-badge'
import { formatDate, formatNumber, formatUSD, formatUSDPlain } from '@/lib/format'
import { msg } from '@/lib/messages'

const MAX_NOTE = 90

export function bookTitle(b: Book): string {
  const t = b.snapshot?.book_title?.trim()
  if (t) return t
  const p = b.prompt.trim()
  if (!p) return msg.library.untitled
  const runes = [...p]
  return runes.length > 48 ? runes.slice(0, 48).join('') + '…' : p
}

function progressPct(b: Book): number {
  const s = b.snapshot
  if (!s || s.total_chapters <= 0) return b.status === 'completed' ? 100 : 0
  return Math.min(100, Math.round((s.completed_count / s.total_chapters) * 100))
}

const barTone: Record<string, string> = {
  running: 'bg-st-running',
  waiting_review: 'bg-st-review',
  budget_exhausted: 'bg-st-budget',
  completed: 'bg-st-done-bg',
  failed: 'bg-st-error',
  interrupted: 'bg-st-error',
}

function note(b: Book): { text: string; cls: string } | null {
  if (b.status === 'waiting_review') {
    return { text: msg.library.waitingNote(b.snapshot?.completed_count ?? 0), cls: 'text-st-review' }
  }
  if (b.status === 'budget_exhausted') return { text: msg.library.budgetUsed, cls: 'text-st-budget' }
  if ((b.status === 'failed' || b.status === 'interrupted') && b.last_error) {
    const runes = [...b.last_error]
    const text = runes.length > MAX_NOTE ? runes.slice(0, MAX_NOTE).join('') + '…' : b.last_error
    return { text, cls: 'text-st-error' }
  }
  return null
}

export function BookCard({ book }: { book: Book }) {
  const s = book.snapshot
  const title = bookTitle(book)
  const n = note(book)
  const pct = progressPct(book)
  const cost = s?.total_cost_usd ?? 0
  const budget = book.budget_usd
  const meta =
    book.status === 'completed'
      ? formatUSD(cost)
      : `${formatUSDPlain(cost)} / ${formatUSD(budget, budget % 1 === 0 ? 0 : 2)}`

  return (
    <Link
      to="/books/$id"
      params={{ id: book.id }}
      aria-label={msg.library.open(title)}
      className="flex flex-col gap-2.5 rounded-[14px] border border-line bg-surface p-4 text-fg no-underline hover:text-fg"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-1">
          <span className="font-serif text-lg leading-snug font-semibold break-words">{title}</span>
          <span className="text-[13px] text-muted">
            {msg.library.started(formatDate(book.created_at))}
          </span>
        </div>
        <StatusBadge status={book.status} />
      </div>
      <div
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={pct}
        className="h-1.5 overflow-hidden rounded-[3px] bg-track"
      >
        <div
          className={`h-1.5 rounded-[3px] ${barTone[book.status] ?? 'bg-st-idle'}`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <div className="flex justify-between gap-3 text-[13px] text-muted">
        <span>
          {msg.library.chapterProgress(s?.completed_count ?? 0, s?.total_chapters ?? 0)}
          {' · '}
          {msg.library.words(formatNumber(s?.total_word_count ?? 0))}
        </span>
        <span className="shrink-0">{meta}</span>
      </div>
      {n && <span className={`text-[13px] font-medium ${n.cls}`}>{n.text}</span>}
    </Link>
  )
}
