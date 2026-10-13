import { Link } from '@tanstack/react-router'
import { ArrowLeft, Download, Pause, Play } from 'lucide-react'
import { useRef, useState, type KeyboardEvent } from 'react'
import { download, errorMessage } from '@/api/client'
import { useBook, useNextChapter, usePatchBook, usePauseBook, useResumeBook } from '@/api/queries'
import type { Book } from '@/api/types.gen'
import { StatusBadge } from '@/components/status-badge'
import { Button, buttonClass } from '@/components/ui/button'
import { FormError } from '@/components/ui/field'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { StreamText } from '@/features/book-stream/stream-text'
import { Timeline } from '@/features/book-stream/timeline'
import {
  selectEvents,
  selectStreamText,
  useBookStreamStore,
  useStreamSelector,
} from '@/features/book-stream/use-book-stream'
import type { BookStreamStore } from '@/features/book-stream/stream-store'
import { formatNumber, formatUSD, formatUSDPlain } from '@/lib/format'
import { msg } from '@/lib/messages'
import { bookTitle } from './book-card'
import { ChaptersTab, OutlineTab } from './book-tabs'
import { BudgetDialog } from './budget-dialog'
import { StatusBanner } from './status-banner'
import { SteerBox } from './steer-box'

type Tab = 'writing' | 'chapters' | 'outline'

const RUNNING = new Set(['running', 'queued'])
const RESUMABLE = new Set(['paused', 'failed', 'interrupted'])

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-xs text-muted">{label}</dt>
      <dd className="m-0 text-[15px] font-semibold">{value}</dd>
    </div>
  )
}

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

function ExportMenu({ bookId, disabled }: { bookId: string; disabled: boolean }) {
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
    'flex min-h-[44px] w-full cursor-pointer items-center border-0 bg-transparent px-4 text-left text-[15px] text-fg hover:bg-track disabled:opacity-60'
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

function ReviewSwitch({ book }: { book: Book }) {
  const patch = usePatchBook(book.id)
  const on = book.advance_mode === 'review'
  return (
    <div className="flex items-center justify-between gap-3">
      <div>
        <span id="review-label" className="text-[15px] font-semibold">{msg.book.reviewMode}</span>
        <p className="m-0 text-[13px] text-muted">{msg.book.reviewModeHint}</p>
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={on}
        aria-labelledby="review-label"
        disabled={patch.isPending}
        onClick={() => patch.mutate({ advance_mode: on ? 'auto' : 'review' })}
        className="flex min-h-[44px] min-w-[52px] shrink-0 cursor-pointer items-center justify-center border-0 bg-transparent p-0"
      >
        <span
          className={`flex h-8 w-[52px] items-center rounded-2xl p-[3px] ${
            on ? 'justify-end bg-accent' : 'justify-start bg-switch-off'
          }`}
        >
          <span className="size-[26px] rounded-full bg-white shadow-sm" />
        </span>
      </button>
    </div>
  )
}

function DraftSection({ store }: { store: BookStreamStore }) {
  const text = useStreamSelector(store, selectStreamText)
  return (
    <section className="rounded-card border border-line bg-surface p-4">
      <h2 className="m-0 mb-2 text-sm font-semibold text-muted">{msg.book.writing.draft}</h2>
      <StreamText text={text} />
    </section>
  )
}

function ActivitySection({ store }: { store: BookStreamStore }) {
  const events = useStreamSelector(store, selectEvents)
  return (
    <section className="rounded-card border border-line bg-surface p-3">
      <h2 className="m-0 mb-1 px-1 text-sm font-semibold text-muted">{msg.book.writing.activity}</h2>
      <Timeline events={events} />
    </section>
  )
}

const tabId = (t: Tab) => `book-tab-${t}`
const panelId = (t: Tab) => `book-panel-${t}`

function Tabs({
  tabs,
  tab,
  onChange,
}: {
  tabs: { id: Tab; label: string }[]
  tab: Tab
  onChange: (t: Tab) => void
}) {
  const refs = useRef(new Map<Tab, HTMLButtonElement>())

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const i = tabs.findIndex((t) => t.id === tab)
    let next: number
    if (e.key === 'ArrowRight') next = (i + 1) % tabs.length
    else if (e.key === 'ArrowLeft') next = (i - 1 + tabs.length) % tabs.length
    else if (e.key === 'Home') next = 0
    else if (e.key === 'End') next = tabs.length - 1
    else return
    e.preventDefault()
    const target = tabs[next]
    if (!target) return
    onChange(target.id)
    refs.current.get(target.id)?.focus()
  }

  return (
    <div role="tablist" aria-label={msg.book.tabs.label} onKeyDown={onKeyDown} className="flex gap-1 rounded-control bg-track p-1">
      {tabs.map((t) => (
        <button
          key={t.id}
          ref={(el) => {
            if (el) refs.current.set(t.id, el)
            else refs.current.delete(t.id)
          }}
          id={tabId(t.id)}
          type="button"
          role="tab"
          aria-selected={tab === t.id}
          aria-controls={panelId(t.id)}
          tabIndex={tab === t.id ? 0 : -1}
          onClick={() => onChange(t.id)}
          className={`min-h-[44px] flex-1 cursor-pointer rounded-[9px] border-0 text-sm ${
            tab === t.id ? 'bg-surface font-semibold text-fg shadow-sm' : 'bg-transparent font-medium text-muted'
          }`}
        >
          {t.label}
        </button>
      ))}
    </div>
  )
}

function Loaded({ book }: { book: Book }) {
  const store = useBookStreamStore(book.id)
  const reconnecting = useStreamSelector(store, (st) => st.reconnecting)
  const failure = useStreamSelector(store, (st) => st.failure)
  const pause = usePauseBook(book.id)
  const resume = useResumeBook(book.id)
  const next = useNextChapter(book.id)
  const [tab, setTab] = useState<Tab>('writing')
  const [budgetOpen, setBudgetOpen] = useState(false)
  const s = book.snapshot
  const busy = pause.isPending || resume.isPending || next.isPending
  const cmdError = pause.error ?? resume.error ?? next.error
  const done = s?.completed_count ?? 0

  const tabs: { id: Tab; label: string }[] = [
    { id: 'writing', label: msg.book.tabs.writing },
    { id: 'chapters', label: done > 0 ? `${msg.book.tabs.chapters} (${done})` : msg.book.tabs.chapters },
    { id: 'outline', label: msg.book.tabs.outline },
  ]

  return (
    <div className="flex min-h-dvh flex-col">
      {failure ? (
        <div role="alert" className="bg-st-error-bg px-4 py-2 text-center text-sm font-medium text-st-error">
          {msg.book.streamStopped}: {failure}
        </div>
      ) : (
        reconnecting && (
        <div role="status" className="bg-st-budget-bg px-4 py-2 text-center text-sm font-medium text-st-budget">
          {msg.book.reconnecting}
        </div>
        )
      )}
      <div className="grid flex-1 grid-cols-1 content-start gap-4 px-4 pb-4 lg:grid-cols-[minmax(0,1fr)_320px] lg:px-6">
        <header className="pt-safe flex items-start gap-2 lg:col-start-1">
          <Link to="/" aria-label={msg.book.back} className="-ml-2 flex size-11 shrink-0 items-center justify-center text-fg">
            <ArrowLeft aria-hidden size={22} />
          </Link>
          <div className="flex min-w-0 flex-1 flex-col gap-1.5">
            <h1 className="m-0 font-serif text-2xl leading-snug font-semibold break-words">{bookTitle(book)}</h1>
            <div className="flex flex-wrap items-center gap-2">
              <StatusBadge status={book.status} />
              {s && s.current_chapter > 0 && (
                <span className="text-[13px] text-muted">
                  {msg.book.chapterOf(
                    s.total_chapters > 0 ? Math.min(s.current_chapter, s.total_chapters) : s.current_chapter,
                    s.total_chapters,
                  )}
                </span>
              )}
            </div>
          </div>
        </header>

        <div className="flex flex-wrap gap-2 lg:col-start-1">
          {RUNNING.has(book.status) && (
            <Button variant="secondary" disabled={busy} onClick={() => pause.mutate()}>
              <Pause aria-hidden size={16} />
              {msg.book.pause}
            </Button>
          )}
          {RESUMABLE.has(book.status) && (
            <Button disabled={busy} onClick={() => resume.mutate()}>
              <Play aria-hidden size={16} />
              {msg.book.resume}
            </Button>
          )}
          <ExportMenu bookId={book.id} disabled={done === 0} />
        </div>

        {cmdError && <div className="lg:col-start-1"><FormError>{errorMessage(cmdError)}</FormError></div>}

        <div className="lg:col-start-1">
          <StatusBanner
            book={book}
            busy={busy}
            onNext={() => next.mutate()}
            onResume={() => resume.mutate()}
            onRaiseBudget={() => setBudgetOpen(true)}
          />
        </div>

        <aside className="flex flex-col gap-4 rounded-card border border-line bg-surface p-4 lg:col-start-2 lg:row-span-4 lg:row-start-1 lg:self-start">
          <dl className="m-0 grid grid-cols-3 gap-3 lg:grid-cols-1">
            <Stat label={msg.book.stats.progress} value={msg.book.chapterOf(done, s?.total_chapters ?? 0)} />
            <Stat label={msg.book.stats.words} value={formatNumber(s?.total_word_count ?? 0)} />
            <Stat
              label={msg.book.stats.cost}
              value={`${formatUSDPlain(s?.total_cost_usd ?? 0)} / ${formatUSD(book.budget_usd, book.budget_usd % 1 === 0 ? 0 : 2)}`}
            />
          </dl>
          <ReviewSwitch book={book} />
        </aside>

        <div className="flex min-w-0 flex-col gap-3 lg:col-start-1">
          <Tabs tabs={tabs} tab={tab} onChange={setTab} />

          {tab === 'writing' && (
            <div role="tabpanel" id={panelId('writing')} aria-labelledby={tabId('writing')} className="flex flex-col gap-4">
              <DraftSection store={store} />
              <ActivitySection store={store} />
            </div>
          )}
          {tab === 'chapters' && (
            <div role="tabpanel" id={panelId('chapters')} aria-labelledby={tabId('chapters')}>
              <ChaptersTab bookId={book.id} />
            </div>
          )}
          {tab === 'outline' && (
            <div role="tabpanel" id={panelId('outline')} aria-labelledby={tabId('outline')}>
              <OutlineTab bookId={book.id} active />
            </div>
          )}
        </div>
      </div>

      <div className="w-full lg:grid lg:px-6 lg:pb-4 lg:grid-cols-[minmax(0,1fr)_320px] lg:gap-4">
        <div className="lg:col-start-1">
          <SteerBox bookId={book.id} pending={book.pending_text} disabled={book.status === 'deleting'} />
        </div>
      </div>

      <BudgetDialog book={book} open={budgetOpen} onClose={() => setBudgetOpen(false)} />
    </div>
  )
}

export function BookScreen({ id }: { id: string }) {
  const book = useBook(id)
  if (book.data) return <Loaded book={book.data} />
  if (book.isPending) return <LoadingState />
  return (
    <div className="p-5">
      <ErrorState title={msg.book.loadError} detail={errorMessage(book.error)} onRetry={() => void book.refetch()} />
    </div>
  )
}
