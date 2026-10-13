import { Link, useNavigate } from '@tanstack/react-router'
import { ArrowLeft, ChevronLeft, ChevronRight, Type } from 'lucide-react'
import { useEffect, useState } from 'react'
import { errorMessage } from '@/api/client'
import { useBook, useChapter, useNextChapter } from '@/api/queries'
import type { Book } from '@/api/types.gen'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { setLastRead } from '@/lib/last-read'
import { msg } from '@/lib/messages'
import { bookTitle } from '../books/book-card'
import { MarkdownView } from './markdown-view'
import { ReadingProgress } from './reading-progress'
import {
  loadSettings,
  MAX_SIZE,
  MIN_SIZE,
  PALETTES,
  systemTheme,
  saveSettings,
  type ReaderFont,
  type ReaderSettings,
  type ReaderTheme,
} from './reader-settings'
import { useReaderShortcuts } from './use-reader-shortcuts'

const THEMES: { id: ReaderTheme; label: string }[] = [
  { id: 'light', label: msg.reader.light },
  { id: 'sepia', label: msg.reader.sepia },
  { id: 'dark', label: msg.reader.dark },
]
const FONTS: { id: ReaderFont; label: string }[] = [
  { id: 'serif', label: msg.reader.serif },
  { id: 'sans', label: msg.reader.sans },
]

const pill = (on: boolean) =>
  `min-h-[44px] flex-1 cursor-pointer rounded-control border-2 bg-transparent px-3 text-sm ${
    on ? 'border-current font-semibold' : 'border-current/20 font-medium'
  }`

function SettingsPanel({
  value,
  onChange,
}: {
  value: ReaderSettings
  /** `pickedTheme`: user bấm nút theme (kể cả theme đang có sẵn theo hệ điều hành). */
  onChange: (s: ReaderSettings, pickedTheme?: boolean) => void
}) {
  return (
    <div role="group" aria-label={msg.reader.settings} className="mx-auto flex w-full max-w-[65ch] flex-col gap-4 py-4">
      <div className="flex items-center gap-3">
        <span className="w-20 text-sm">{msg.reader.fontSize}</span>
        <button
          type="button"
          aria-label={msg.reader.smaller}
          disabled={value.size <= MIN_SIZE}
          onClick={() => onChange({ ...value, size: value.size - 1 })}
          className={pill(false)}
        >
          A−
        </button>
        <span className="w-14 text-center text-sm">{value.size}px</span>
        <button
          type="button"
          aria-label={msg.reader.larger}
          disabled={value.size >= MAX_SIZE}
          onClick={() => onChange({ ...value, size: value.size + 1 })}
          className={pill(false)}
        >
          A+
        </button>
      </div>
      <div className="flex items-center gap-3">
        <span className="w-20 text-sm">{msg.reader.fontFamily}</span>
        {FONTS.map((f) => (
          <button key={f.id} type="button" aria-pressed={value.font === f.id} onClick={() => onChange({ ...value, font: f.id })} className={pill(value.font === f.id)}>
            {f.label}
          </button>
        ))}
      </div>
      <div className="flex items-center gap-3">
        <span className="w-20 text-sm">{msg.reader.theme}</span>
        {THEMES.map((t) => (
          <button key={t.id} type="button" aria-pressed={value.theme === t.id} onClick={() => onChange({ ...value, theme: t.id }, true)} className={pill(value.theme === t.id)}>
            {t.label}
          </button>
        ))}
      </div>
    </div>
  )
}

const navClass =
  'inline-flex min-h-[44px] cursor-pointer items-center gap-1 rounded-control border-2 border-current/25 bg-transparent px-4 text-base font-medium no-underline disabled:cursor-default disabled:opacity-60'

const WRITING = new Set(['running', 'queued'])

/**
 * Cuối chương mới nhất: báo đây là chương cuối; truyện chờ duyệt thì cho viết chương tiếp ngay,
 * truyện đang chạy thì báo AI đang viết chương nào và dẫn về trang truyện.
 */
export function LatestChapterNote({ book, chapter, color }: { book: Book | undefined; chapter: number; color: string }) {
  const next = useNextChapter(book?.id ?? '')
  const status = book?.status ?? ''
  const writing = book?.snapshot?.in_progress_chapter || chapter + 1
  return (
    <section aria-label={msg.reader.last} className="mx-auto mt-10 flex w-full max-w-[65ch] flex-col items-start gap-3 border-t border-current/20 pt-6">
      <p className="m-0 font-semibold">{msg.reader.last}</p>
      {book && WRITING.has(status) ? (
        <p className="m-0 flex flex-wrap items-center gap-x-3 gap-y-2">
          <span role="status">{msg.reader.aiWriting(writing)}</span>
          <Link to="/books/$id" params={{ id: book.id }} className="underline" style={{ color }}>
            {msg.reader.watch}
          </Link>
        </p>
      ) : (
        book &&
        status === 'waiting_review' && (
          <>
            <button
              type="button"
              className={navClass}
              style={{ color }}
              disabled={next.isPending || next.isSuccess}
              onClick={() => next.mutate()}
            >
              {msg.reader.writeNext}
            </button>
            {next.error && (
              <p role="alert" className="m-0 text-sm">
                {errorMessage(next.error)}
              </p>
            )}
          </>
        )
      )}
    </section>
  )
}

export function ReaderPage({ bookId, n }: { bookId: string; n: number }) {
  const chapter = useChapter(bookId, n)
  const book = useBook(bookId)
  const navigate = useNavigate()
  const [stored] = useState(loadSettings)
  const [settings, setSettings] = useState<ReaderSettings>(stored.settings)
  const [themeChosen, setThemeChosen] = useState(stored.themeChosen)
  const [panel, setPanel] = useState(false)
  const p = PALETTES[settings.theme]
  const c = chapter.data
  const validN = Number.isInteger(n) && n > 0

  useEffect(() => {
    window.scrollTo(0, 0)
  }, [n])

  // Chưa tự chọn theme thì reader đi theo hệ điều hành, kể cả khi OS đổi sáng/tối lúc đang đọc.
  useEffect(() => {
    if (themeChosen || typeof window.matchMedia !== 'function') return
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const follow = () => setSettings((s) => ({ ...s, theme: systemTheme() }))
    mq.addEventListener('change', follow)
    return () => mq.removeEventListener('change', follow)
  }, [themeChosen])

  // Nhớ chỗ đọc khi chương tải xong (thẻ thư viện và trang truyện hiện "Đọc tiếp chương N").
  const loaded = c?.number
  useEffect(() => {
    if (loaded) setLastRead(bookId, loaded)
  }, [bookId, loaded])

  const toChapter = (to: number) => () => void navigate({ to: '/books/$id/read/$n', params: { id: bookId, n: String(to) } })
  useReaderShortcuts({
    onPrev: c && c.prev > 0 ? toChapter(c.prev) : undefined,
    onNext: c && c.next > 0 ? toChapter(c.next) : undefined,
    onExit: () => void navigate({ to: '/books/$id', params: { id: bookId } }),
    disabled: panel,
  })

  const update = (s: ReaderSettings, pickedTheme = false) => {
    const chosen = themeChosen || pickedTheme
    setSettings(s)
    setThemeChosen(chosen)
    saveSettings(s, chosen)
  }

  const title = book.data ? bookTitle(book.data) : ''
  const heading = validN ? msg.reader.chapterHeading(n, c?.title ?? '') : ''

  return (
    <div className="flex min-h-dvh flex-col" style={{ background: p.bg, color: p.fg }}>
      <header className="pt-safe sticky top-0 z-10 px-3 pb-2" style={{ background: p.bg }}>
        {validN && c && <ReadingProgress />}
        <div className="mx-auto flex w-full max-w-[65ch] items-center gap-2">
          <Link
            to="/books/$id"
            params={{ id: bookId }}
            aria-label={msg.reader.back}
            aria-keyshortcuts="Escape"
            className="flex size-11 shrink-0 items-center justify-center no-underline"
            style={{ color: p.fg }}
          >
            <ArrowLeft aria-hidden size={22} />
          </Link>
          {/* Tên truyện cắt gọn trước, phần chương giữ được nhiều chữ nhất có thể. */}
          <p className="m-0 flex min-w-0 flex-1 text-sm opacity-70">
            {title && (
              <>
                <span className="max-w-[45%] shrink-0 truncate">{title}</span>
                <span aria-hidden className="shrink-0 whitespace-pre"> · </span>
              </>
            )}
            <span className="min-w-0 truncate">{heading}</span>
          </p>
          <button
            type="button"
            aria-label={msg.reader.settings}
            aria-expanded={panel}
            onClick={() => setPanel((v) => !v)}
            className="flex size-11 cursor-pointer items-center justify-center rounded-control border-0 bg-transparent"
            style={{ color: p.fg }}
          >
            <Type aria-hidden size={20} />
          </button>
        </div>
        {panel && (
          <div className="border-t px-1" style={{ borderColor: 'color-mix(in srgb, currentColor 20%, transparent)' }}>
            <SettingsPanel value={settings} onChange={update} />
          </div>
        )}
      </header>

      <main className="flex-1 px-5 pb-8">
        {!validN ? (
          <div className="mx-auto max-w-[65ch] py-6">
            <ErrorState title={msg.reader.loadError} detail={msg.notFound} />
          </div>
        ) : chapter.isPending ? (
          <LoadingState />
        ) : chapter.isError || !c ? (
          <div className="mx-auto max-w-[65ch] py-6">
            <ErrorState title={msg.reader.loadError} detail={errorMessage(chapter.error)} onRetry={() => void chapter.refetch()} />
          </div>
        ) : (
          <>
            <article
              className="reader-prose mx-auto w-full max-w-[65ch]"
              style={{
                fontSize: `${settings.size}px`,
                lineHeight: 1.75,
                fontFamily: settings.font === 'serif' ? 'var(--font-serif)' : 'var(--font-ui)',
              }}
            >
              <h1 className="mb-6 text-[1.5em]">{c.title}</h1>
              <MarkdownView content={c.content} />
            </article>
            {c.next === 0 && <LatestChapterNote book={book.data} chapter={c.number} color={p.fg} />}
          </>
        )}
      </main>

      <footer className="pb-safe px-5 pt-2">
        <nav aria-label={validN ? msg.reader.chapterLabel(n) : msg.reader.back} className="mx-auto flex w-full max-w-[65ch] items-center justify-between gap-3">
          {c && c.prev > 0 ? (
            <Link
              to="/books/$id/read/$n"
              params={{ id: bookId, n: String(c.prev) }}
              aria-keyshortcuts="ArrowLeft"
              className={navClass}
              style={{ color: p.fg }}
            >
              <ChevronLeft aria-hidden size={18} />
              {msg.reader.prev}
            </Link>
          ) : (
            <span />
          )}
          {c && c.next > 0 ? (
            <Link
              to="/books/$id/read/$n"
              params={{ id: bookId, n: String(c.next) }}
              aria-keyshortcuts="ArrowRight"
              className={navClass}
              style={{ color: p.fg }}
            >
              {msg.reader.next}
              <ChevronRight aria-hidden size={18} />
            </Link>
          ) : c ? (
            <Link to="/books/$id" params={{ id: bookId }} aria-keyshortcuts="Escape" className={navClass} style={{ color: p.fg }}>
              {msg.reader.back}
            </Link>
          ) : null}
        </nav>
      </footer>
    </div>
  )
}
