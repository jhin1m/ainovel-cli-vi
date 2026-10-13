import { Link } from '@tanstack/react-router'
import { errorMessage } from '@/api/client'
import { useChapters, useOutline } from '@/api/queries'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { formatNumber } from '@/lib/format'
import { msg } from '@/lib/messages'

export function ChaptersTab({ bookId }: { bookId: string }) {
  const q = useChapters(bookId)
  if (q.isPending) return <LoadingState />
  if (q.isError) return <ErrorState title={msg.book.chaptersTab.loadError} detail={errorMessage(q.error)} onRetry={() => void q.refetch()} />
  if (q.data.length === 0) return <EmptyState title={msg.book.chaptersTab.empty} />
  return (
    <ul className="m-0 flex list-none flex-col gap-2 p-0">
      {q.data.map((c) => (
        <li key={c.number}>
          <Link
            to="/books/$id/read/$n"
            params={{ id: bookId, n: String(c.number) }}
            aria-label={msg.book.chaptersTab.open(c.number, c.title)}
            className="flex min-h-[44px] items-center justify-between gap-3 rounded-card border border-line bg-surface px-4 py-3 text-fg no-underline hover:text-fg"
          >
            <span className="min-w-0 font-serif break-words">
              <span className="text-muted">{c.number}.</span> {c.title}
            </span>
            <span className="shrink-0 text-sm text-muted">{msg.book.chaptersTab.words(formatNumber(c.word_count))}</span>
          </Link>
        </li>
      ))}
    </ul>
  )
}

export function OutlineTab({ bookId, active }: { bookId: string; active: boolean }) {
  const q = useOutline(bookId, active)
  if (q.isPending) return q.fetchStatus === 'idle' ? null : <LoadingState />
  if (q.isError) return <ErrorState title={msg.book.outlineTab.loadError} detail={errorMessage(q.error)} onRetry={() => void q.refetch()} />
  const o = q.data
  const volumes = o.volumes ?? []
  const characters = o.characters ?? []
  if (!o.synopsis && volumes.every((v) => (v.chapters ?? []).length === 0) && characters.length === 0) {
    return <EmptyState title={msg.book.outlineTab.empty} />
  }
  return (
    <div className="flex flex-col gap-5">
      {o.synopsis && (
        <section>
          <h3 className="m-0 mb-1 font-serif text-lg">{msg.book.outlineTab.synopsis}</h3>
          <p className="m-0 text-base leading-relaxed break-words whitespace-pre-wrap">{o.synopsis}</p>
        </section>
      )}
      {characters.length > 0 && (
        <section>
          <h3 className="m-0 mb-2 font-serif text-lg">{msg.book.outlineTab.characters}</h3>
          <ul className="m-0 flex list-none flex-col gap-2 p-0">
            {characters.map((c, i) => (
              <li key={i} className="rounded-card border border-line bg-surface p-3 text-sm break-words">
                <span className="font-semibold">{c.name}</span>
                {c.role && <span className="text-muted"> · {c.role}</span>}
                {c.description && <p className="m-0 mt-1 text-muted">{c.description}</p>}
              </li>
            ))}
          </ul>
        </section>
      )}
      <section>
        <h3 className="m-0 mb-2 font-serif text-lg">{msg.book.outlineTab.chapters}</h3>
        {volumes.map((v, i) => (
          <div key={i} className="mb-3">
            {v.title && <h4 className="m-0 mb-1 text-base font-semibold">{v.title}</h4>}
            <ol className="m-0 flex list-none flex-col gap-1.5 p-0">
              {(v.chapters ?? []).map((c) => (
                <li key={c.number} className="rounded-control bg-surface px-3 py-2 text-sm break-words">
                  <span className="font-semibold">{c.number}. {c.title}</span>
                  {c.done && <span className="ml-2 text-xs text-st-running">{msg.book.outlineTab.done}</span>}
                  {c.core_event && <p className="m-0 mt-0.5 text-muted">{c.core_event}</p>}
                </li>
              ))}
            </ol>
          </div>
        ))}
      </section>
    </div>
  )
}
