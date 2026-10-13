import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { errorMessage } from '@/api/client'
import { useBooks } from '@/api/queries'
import type { Book } from '@/api/types.gen'
import { BookOpen } from 'lucide-react'
import { buttonClass } from '@/components/ui/button'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { msg } from '@/lib/messages'
import { BookCard } from './book-card'

type Filter = keyof typeof msg.library.filters

const matches: Record<Filter, (b: Book) => boolean> = {
  all: () => true,
  running: (b) => b.status === 'running' || b.status === 'queued',
  review: (b) => b.status === 'waiting_review',
  done: (b) => b.status === 'completed',
}

export function LibraryPage() {
  const books = useBooks()
  const [filter, setFilter] = useState<Filter>('all')

  const list = books.data ?? []
  const visible = list.filter(matches[filter])
  const running = list.filter(matches.running).length
  const waiting = list.filter(matches.review).length

  return (
    <>
      <header className="pt-safe flex items-center gap-2.5 px-5 pb-2">
        <BookOpen aria-hidden className="text-accent" size={28} strokeWidth={1.8} />
        <span className="text-[19px] font-bold tracking-tight">{msg.appName}</span>
      </header>
      <section className="px-5 pt-3 pb-1">
        <h1 className="m-0 font-serif text-[32px] font-semibold tracking-tight">{msg.library.title}</h1>
        {books.isSuccess && list.length > 0 && (
          <p className="mt-1.5 mb-0 text-sm text-muted">
            {msg.library.summary(list.length, running, waiting)}
          </p>
        )}
      </section>

      {books.isSuccess && list.length > 0 && (
        <nav aria-label={msg.library.filterLabel} className="flex gap-2 overflow-x-auto px-5 pt-3.5 pb-1.5">
          {(Object.keys(msg.library.filters) as Filter[]).map((f) => (
            <button
              key={f}
              type="button"
              aria-pressed={filter === f}
              onClick={() => setFilter(f)}
              className={`min-h-11 shrink-0 cursor-pointer rounded-full border px-4 text-sm font-medium ${
                filter === f ? 'border-fg bg-fg text-bg' : 'border-line bg-surface text-fg'
              }`}
            >
              {msg.library.filters[f]}
            </button>
          ))}
        </nav>
      )}

      <main className="flex flex-col gap-3 px-5 pt-2.5 pb-5">
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
              <Link to="/books/new" className={buttonClass('primary', 'no-underline hover:text-on-accent text-on-accent')}>
                {msg.nav.newBook}
              </Link>
            }
          />
        ) : visible.length === 0 ? (
          <p className="m-0 py-8 text-center text-sm text-muted">{msg.library.noMatch}</p>
        ) : (
          visible.map((b) => <BookCard key={b.id} book={b} />)
        )}
      </main>
    </>
  )
}
