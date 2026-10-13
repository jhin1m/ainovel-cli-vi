import { Link } from '@tanstack/react-router'
import { ArrowLeft, ChevronLeft, ChevronRight, Type } from 'lucide-react'
import { useEffect, useState } from 'react'
import { errorMessage } from '@/api/client'
import { useChapter } from '@/api/queries'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { msg } from '@/lib/messages'
import { MarkdownView } from './markdown-view'
import {
  loadSettings,
  MAX_SIZE,
  MIN_SIZE,
  PALETTES,
  saveSettings,
  type ReaderFont,
  type ReaderSettings,
  type ReaderTheme,
} from './reader-settings'

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
  `min-h-[44px] flex-1 cursor-pointer rounded-[10px] border-2 bg-transparent px-3 text-sm ${
    on ? 'border-current font-semibold' : 'border-current/20 font-medium'
  }`

function SettingsPanel({ value, onChange }: { value: ReaderSettings; onChange: (s: ReaderSettings) => void }) {
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
          <button key={t.id} type="button" aria-pressed={value.theme === t.id} onClick={() => onChange({ ...value, theme: t.id })} className={pill(value.theme === t.id)}>
            {t.label}
          </button>
        ))}
      </div>
    </div>
  )
}

export function ReaderPage({ bookId, n }: { bookId: string; n: number }) {
  const chapter = useChapter(bookId, n)
  const [settings, setSettings] = useState<ReaderSettings>(loadSettings)
  const [panel, setPanel] = useState(false)
  const p = PALETTES[settings.theme]

  useEffect(() => {
    window.scrollTo(0, 0)
  }, [n])

  const update = (s: ReaderSettings) => {
    setSettings(s)
    saveSettings(s)
  }

  const nav = 'inline-flex min-h-[44px] items-center gap-1 rounded-[10px] border-2 border-current/25 px-4 text-[15px] font-medium no-underline'
  const c = chapter.data

  return (
    <div className="flex min-h-dvh flex-col" style={{ background: p.bg, color: p.fg }}>
      <header className="pt-safe sticky top-0 z-10 px-3 pb-2" style={{ background: p.bg }}>
        <div className="mx-auto flex w-full max-w-[65ch] items-center gap-2">
          <Link
            to="/books/$id"
            params={{ id: bookId }}
            aria-label={msg.reader.back}
            className="flex size-11 shrink-0 items-center justify-center no-underline"
            style={{ color: p.fg }}
          >
            <ArrowLeft aria-hidden size={22} />
          </Link>
          <span className="min-w-0 flex-1 truncate text-sm opacity-70">{msg.reader.chapterLabel(n)}</span>
          <button
            type="button"
            aria-label={msg.reader.settings}
            aria-expanded={panel}
            onClick={() => setPanel((v) => !v)}
            className="flex size-11 cursor-pointer items-center justify-center rounded-[10px] border-0 bg-transparent"
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
        {chapter.isPending ? (
          <LoadingState />
        ) : chapter.isError || !c ? (
          <div className="mx-auto max-w-[65ch] py-6">
            <ErrorState title={msg.reader.loadError} detail={errorMessage(chapter.error)} onRetry={() => void chapter.refetch()} />
          </div>
        ) : (
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
        )}
      </main>

      <footer className="pb-safe px-5 pt-2">
        <nav aria-label={msg.reader.chapterLabel(n)} className="mx-auto flex w-full max-w-[65ch] items-center justify-between gap-3">
          {c && c.prev > 0 ? (
            <Link to="/books/$id/read/$n" params={{ id: bookId, n: String(c.prev) }} className={nav} style={{ color: p.fg }}>
              <ChevronLeft aria-hidden size={18} />
              {msg.reader.prev}
            </Link>
          ) : (
            <span />
          )}
          {c && c.next > 0 ? (
            <Link to="/books/$id/read/$n" params={{ id: bookId, n: String(c.next) }} className={nav} style={{ color: p.fg }}>
              {msg.reader.next}
              <ChevronRight aria-hidden size={18} />
            </Link>
          ) : c ? (
            <Link to="/books/$id" params={{ id: bookId }} className={nav} style={{ color: p.fg }}>
              {msg.reader.back}
            </Link>
          ) : null}
        </nav>
      </footer>
    </div>
  )
}
