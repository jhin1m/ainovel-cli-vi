import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { errorMessage } from '@/api/client'
import { useBooks, useKeys } from '@/api/queries'
import type { Book } from '@/api/types.gen'
import { BookOpen, Plus } from 'lucide-react'
import { statusLabel } from '@/components/status-badge'
import { buttonClass } from '@/components/ui/button'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { GettingStarted } from '@/features/onboarding/getting-started'
import { RunLocally } from '@/features/onboarding/run-locally'
import { msg } from '@/lib/messages'
import { needsAttention } from '@/lib/needs-attention'
import { BookCard, bookTitle } from './book-card'
import type { BookAction } from './book-screen'

type Filter = keyof typeof msg.library.filters

const matches: Record<Filter, (b: Book) => boolean> = {
  all: () => true,
  running: (b) => b.status === 'running' || b.status === 'queued',
  review: (b) => b.status === 'waiting_review',
  done: (b) => b.status === 'completed',
}

const primaryLink = buttonClass('primary', 'no-underline text-on-accent hover:text-on-accent')

/** Nút của dải mở trang truyện kèm hành động: trang tự mở hộp thoại ngân sách hoặc chạy lại. */
const ATTENTION_ACTION: Partial<Record<string, BookAction>> = {
  budget_exhausted: 'budget',
  interrupted: 'retry',
}

/** Dải "Cần bạn xử lý": cùng bộ lọc với tiền tố "(n)" trên tab. Ẩn khi rỗng. */
function AttentionStrip({ books }: { books: readonly Book[] }) {
  const list = needsAttention(books)
  if (list.length === 0) return null
  const t = msg.library.attention
  return (
    <section aria-labelledby="attention-title" className="callout flex flex-col gap-2">
      <h2 id="attention-title" className="m-0 font-serif text-lg font-semibold">
        {t.title(list.length)}
      </h2>
      <ul className="m-0 flex list-none flex-col gap-1 p-0">
        {list.map((b) => {
          const title = bookTitle(b)
          const action = t.action[b.status] ?? t.fallbackAction
          return (
            <li key={b.id} className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
              <span className="min-w-0 flex-1 basis-48">
                <span className="font-medium break-words">{title}</span>
                <span className="text-sm text-muted"> · {statusLabel(b.status)}</span>
              </span>
              <Link
                to="/books/$id"
                params={{ id: b.id }}
                search={{ action: ATTENTION_ACTION[b.status] }}
                aria-label={t.rowLabel(title, action)}
                className={buttonClass('secondary', 'min-h-11 shrink-0 px-4 text-sm text-fg no-underline hover:text-fg')}
              >
                {action}
              </Link>
            </li>
          )
        })}
      </ul>
    </section>
  )
}

export function LibraryPage() {
  const books = useBooks()
  const keys = useKeys()
  const [filter, setFilter] = useState<Filter>('all')

  const list = books.data ?? []
  const visible = list.filter(matches[filter])
  const running = list.filter(matches.running).length
  const waiting = list.filter(matches.review).length
  const hasBooks = books.isSuccess && list.length > 0

  return (
    <main className="flex flex-col gap-5 px-4 pb-6 md:px-0">
      {/* Thương hiệu chỉ hiện dưới lg; desktop đã có ở sidebar. */}
      <div className="pt-safe flex items-center gap-2.5 lg:hidden">
        <BookOpen aria-hidden className="text-accent" size={28} strokeWidth={1.8} />
        <span className="text-xl font-bold tracking-tight">{msg.appName}</span>
      </div>

      <header className="flex flex-wrap items-end justify-between gap-3 lg:pt-6">
        <div className="flex min-w-0 flex-col gap-1">
          <h1 className="m-0 font-serif text-2xl font-semibold tracking-tight md:text-3xl">{msg.library.title}</h1>
          {hasBooks && (
            <p className="m-0 text-sm text-muted">{msg.library.summary(list.length, running, waiting)}</p>
          )}
        </div>
        {hasBooks && (
          <Link to="/books/new" className={primaryLink}>
            <Plus aria-hidden size={18} />
            {msg.library.newBook}
          </Link>
        )}
      </header>

      {books.isSuccess && keys.isSuccess && <GettingStarted keys={keys.data} books={list} />}

      {hasBooks && <AttentionStrip books={list} />}

      {hasBooks && (
        <nav aria-label={msg.library.filterLabel} className="-mx-4 flex gap-2 overflow-x-auto px-4 md:mx-0 md:px-0">
          {(Object.keys(msg.library.filters) as Filter[]).map((f) => (
            <button
              key={f}
              type="button"
              aria-pressed={filter === f}
              onClick={() => setFilter(f)}
              className={`min-h-11 shrink-0 cursor-pointer rounded-pill border px-4 text-sm font-medium ${
                filter === f ? 'border-fg bg-fg text-bg' : 'border-line bg-surface text-fg'
              }`}
            >
              {msg.library.filters[f]}
            </button>
          ))}
        </nav>
      )}

      <div className="flex flex-col gap-5">
        {books.isPending ? (
          <LoadingState />
        ) : books.isError ? (
          <ErrorState
            title={msg.library.loadError}
            detail={errorMessage(books.error)}
            onRetry={() => void books.refetch()}
          />
        ) : list.length === 0 ? (
          <EmptyState
            title={msg.library.empty}
            hint={msg.library.emptyHint}
            action={
              <Link to="/books/new" className={primaryLink}>
                {msg.nav.newBook}
              </Link>
            }
          />
        ) : visible.length === 0 ? (
          <p className="m-0 py-8 text-center text-sm text-muted">{msg.library.noMatch}</p>
        ) : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 min-[90rem]:grid-cols-4">
            {visible.map((b) => (
              <BookCard key={b.id} book={b} />
            ))}
          </div>
        )}
        {!books.isPending && <RunLocally />}
      </div>
    </main>
  )
}
