import { Link } from '@tanstack/react-router'
import { errorMessage } from '@/api/client'
import { useNextChapter } from '@/api/queries'
import type { Book } from '@/api/types.gen'
import { ProgressRing } from '@/components/progress-ring'
import { ProviderIcon } from '@/components/provider-icon'
import { StatusBadge, statusLabel } from '@/components/status-badge'
import { Button, buttonClass } from '@/components/ui/button'
import { coverTone, type CoverTone } from '@/lib/cover-tone'
import { formatDate, formatNumber, formatUSD, formatUSDPlain } from '@/lib/format'
import { getLastRead } from '@/lib/last-read'
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

// Lớp Tailwind viết nguyên chuỗi để bộ quét không bỏ sót.
const coverBg: Record<CoverTone, string> = {
  1: 'bg-cover-1',
  2: 'bg-cover-2',
  3: 'bg-cover-3',
  4: 'bg-cover-4',
  5: 'bg-cover-5',
  6: 'bg-cover-6',
}

const ringTone: Record<string, string> = {
  running: 'text-st-running',
  waiting_review: 'text-st-review',
  budget_exhausted: 'text-st-budget',
  // Badge hoàn thành dùng nền đậm và chữ nhạt, nên vòng lấy màu nền của badge.
  completed: 'text-st-done-bg',
  failed: 'text-st-error',
  interrupted: 'text-st-error',
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

/** Dòng chi phí: truyện thuê bao hiện token, còn lại hiện USD (đã dùng / ngân sách). */
function costLine(b: Book): string {
  const s = b.snapshot
  if (b.subscription) {
    return msg.library.tokens(formatNumber((s?.total_input_tokens ?? 0) + (s?.total_output_tokens ?? 0)))
  }
  const cost = s?.total_cost_usd ?? 0
  if (b.status === 'completed') return formatUSD(cost)
  const budget = b.budget_usd
  return `${formatUSDPlain(cost)} / ${formatUSD(budget, budget % 1 === 0 ? 0 : 2)}`
}

/** Tên model ngắn: bỏ tiền tố hãng của OpenRouter ("anthropic/claude-…" → "claude-…"). */
export function shortModel(model: string): string {
  return model.split('/').pop() || model
}

export type QuickAction =
  | { kind: 'read'; label: string; chapter: number }
  | { kind: 'book'; label: string }
  | { kind: 'next'; label: string }

/**
 * Nút nhanh theo trạng thái. Tối đa 2 nút để thẻ gọn:
 * chờ duyệt → đọc chương vừa xong + viết tiếp; hoàn thành → đọc từ đầu (hoặc đọc tiếp nếu đã đọc dở);
 * lỗi → xem lỗi; còn lại → đọc tiếp nếu đã đọc dở.
 */
export function quickActions(b: Book, lastRead: number | null): QuickAction[] {
  const a = msg.library.actions
  const done = b.snapshot?.completed_count ?? 0
  // Chỉ nhắc chương đã đọc nếu chương đó còn tồn tại.
  const resume: QuickAction | null =
    lastRead !== null && lastRead <= done ? { kind: 'read', label: a.continueReading(lastRead), chapter: lastRead } : null

  switch (b.status) {
    case 'waiting_review':
      return [
        ...(done > 0 ? [{ kind: 'read', label: a.readChapter(done), chapter: done } as const] : []),
        { kind: 'next', label: a.next },
      ]
    case 'completed':
      if (resume) return [resume]
      return done > 0 ? [{ kind: 'read', label: a.readFromStart, chapter: 1 }] : []
    case 'failed':
    case 'interrupted':
      return [{ kind: 'book', label: a.viewError }, ...(resume ? [resume] : [])]
    default:
      return resume ? [resume] : []
  }
}

function NextButton({ book, label }: { book: Book; label: string }) {
  const next = useNextChapter(book.id)
  return (
    <>
      <Button className="min-h-11 px-4 text-sm" disabled={next.isPending || next.isSuccess} onClick={() => next.mutate()}>
        {label}
      </Button>
      {next.error && (
        <p role="alert" className="m-0 basis-full text-sm text-st-error">
          {errorMessage(next.error)}
        </p>
      )}
    </>
  )
}

export function BookCard({ book }: { book: Book }) {
  const s = book.snapshot
  const title = bookTitle(book)
  const n = note(book)
  const pct = progressPct(book)
  const done = s?.completed_count ?? 0
  const total = s?.total_chapters ?? 0
  const model = shortModel(book.model)
  const actions = quickActions(book, getLastRead(book.id))
  const secondary = buttonClass('secondary', 'min-h-11 px-4 text-sm text-fg no-underline hover:text-fg')

  return (
    <article className="card-hover relative flex flex-col overflow-hidden rounded-card border border-line-card bg-surface shadow-card">
      <div aria-hidden data-cover className={`h-1 ${coverBg[coverTone(book.id)]}`} />
      <div className="flex flex-1 flex-col gap-3 p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="flex min-w-0 flex-col gap-1">
            <h3 className="m-0 font-serif text-lg leading-snug font-semibold break-words">
              {/* Lớp ::after phủ cả thẻ để bấm chỗ nào cũng mở truyện; các nút nằm trên lớp đó (z-10). */}
              <Link
                to="/books/$id"
                params={{ id: book.id }}
                aria-label={msg.library.open(
                  title,
                  statusLabel(book.status),
                  msg.library.chapterProgress(done, total),
                )}
                className="text-fg no-underline after:absolute after:inset-0 after:content-[''] hover:text-fg focus-visible:outline-none after:rounded-card focus-visible:after:outline-2 focus-visible:after:-outline-offset-2 focus-visible:after:outline-accent"
              >
                {title}
              </Link>
            </h3>
            <span className="text-sm text-muted">{msg.library.started(formatDate(book.created_at))}</span>
          </div>
          <StatusBadge status={book.status} />
        </div>

        <div className="flex items-center gap-4">
          <ProgressRing value={pct} className={ringTone[book.status] ?? 'text-st-idle'} />
          <dl className="m-0 flex flex-wrap gap-x-5 gap-y-1">
            <div className="flex flex-col">
              <dt className="order-2 text-sm text-muted">{msg.library.chaptersUnit}</dt>
              <dd className="m-0 font-serif text-xl font-semibold tabular-nums">
                {msg.library.chaptersBig(done, total)}
              </dd>
            </div>
            <div className="flex flex-col">
              <dt className="order-2 text-sm text-muted">{msg.library.wordsUnit}</dt>
              <dd className="m-0 font-serif text-xl font-semibold tabular-nums">
                {formatNumber(s?.total_word_count ?? 0)}
              </dd>
            </div>
          </dl>
        </div>

        <div className="flex items-center justify-between gap-3 text-sm text-muted">
          <span className="flex min-w-0 items-center gap-1.5">
            <ProviderIcon name={book.provider} size={16} />
            <span className="sr-only">{msg.library.providerModel(book.provider, model)}</span>
            <span aria-hidden className="truncate">
              {model}
            </span>
          </span>
          <span className="shrink-0 tabular-nums">{costLine(book)}</span>
        </div>

        {n && <span className={`text-sm font-medium ${n.cls}`}>{n.text}</span>}

        {actions.length > 0 && (
          <div className="relative z-10 mt-auto flex flex-wrap gap-2">
            {actions.map((a) =>
              a.kind === 'next' ? (
                <NextButton key="next" book={book} label={a.label} />
              ) : a.kind === 'read' ? (
                <Link
                  key={'read' + a.chapter}
                  to="/books/$id/read/$n"
                  params={{ id: book.id, n: String(a.chapter) }}
                  className={secondary}
                >
                  {a.label}
                </Link>
              ) : (
                <Link key="book" to="/books/$id" params={{ id: book.id }} className={secondary}>
                  {a.label}
                </Link>
              ),
            )}
          </div>
        )}
      </div>
    </article>
  )
}
