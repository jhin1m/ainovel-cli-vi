import { Link, type LinkProps } from '@tanstack/react-router'
import { Check } from 'lucide-react'
import type { APIKey, Book } from '@/api/types.gen'
import { buttonClass } from '@/components/ui/button'
import { msg } from '@/lib/messages'

type StepId = keyof typeof msg.onboarding.steps

interface Step {
  id: StepId
  done: boolean
  /** Không có khi bước chưa làm được (bước 3 khi chưa có truyện nào). */
  link?: LinkProps
}

/** Truyện tạo gần nhất, để bước 3 dẫn thẳng tới đó. */
function newest(books: readonly Book[]): Book | undefined {
  let best: Book | undefined
  for (const b of books) if (!best || b.created_at > best.created_at) best = b
  return best
}

/** Tính 3 bước từ dữ liệu thật. Không lưu cờ nào: dữ liệu đổi thì checklist tự đổi theo. */
export function gettingStartedSteps(keys: readonly APIKey[], books: readonly Book[]): Step[] {
  const latest = newest(books)
  const read = books.find((b) => (b.snapshot?.completed_count ?? 0) >= 1)
  const target = read ?? latest
  return [
    { id: 'key', done: keys.length > 0, link: { to: '/settings/keys' } },
    { id: 'book', done: books.length > 0, link: { to: '/books/new' } },
    {
      id: 'read',
      done: read !== undefined,
      link: target ? { to: '/books/$id', params: { id: target.id } } : undefined,
    },
  ]
}

/** Checklist "Bắt đầu trong 3 bước". Ẩn hẳn khi đã xong cả 3 bước. */
export function GettingStarted({ keys, books }: { keys: readonly APIKey[]; books: readonly Book[] }) {
  const steps = gettingStartedSteps(keys, books)
  const doneCount = steps.filter((s) => s.done).length
  if (doneCount === steps.length) return null
  const current = steps.findIndex((s) => !s.done)
  const t = msg.onboarding

  return (
    <section
      aria-labelledby="getting-started-title"
      className="flex flex-col gap-3 rounded-card border border-line-card bg-surface p-4 shadow-card md:p-5"
    >
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 id="getting-started-title" className="m-0 font-serif text-xl font-semibold">
          {t.title}
        </h2>
        <span className="text-sm text-muted">{t.progress(doneCount)}</span>
      </div>
      <ol className="m-0 flex list-none flex-col gap-2 p-0">
        {steps.map((s, i) => {
          const copy = t.steps[s.id]
          const isCurrent = i === current
          return (
            <li
              key={s.id}
              data-step={s.id}
              aria-current={isCurrent ? 'step' : undefined}
              className={`flex flex-wrap items-start gap-3 ${isCurrent ? 'callout' : 'border-l-3 border-transparent px-4 py-3'}`}
            >
              <span
                aria-hidden
                className={`mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full text-xs font-bold ${
                  s.done ? 'bg-accent text-on-accent' : 'border border-line-input text-muted'
                }`}
              >
                {s.done ? <Check size={14} strokeWidth={3} /> : i + 1}
              </span>
              <div className="flex min-w-0 flex-1 basis-48 flex-col gap-1">
                <span className={`font-semibold ${s.done ? 'text-muted line-through' : ''}`}>
                  {copy.title}
                  {s.done && <span className="sr-only"> ({t.done})</span>}
                  {isCurrent && <span className="sr-only"> ({t.current})</span>}
                </span>
                {!s.done && <span className="text-sm text-muted">{copy.hint}</span>}
              </div>
              {!s.done && s.link && (
                <Link
                  {...s.link}
                  className={buttonClass(
                    isCurrent ? 'primary' : 'secondary',
                    `shrink-0 no-underline ${isCurrent ? 'text-on-accent hover:text-on-accent' : 'text-fg hover:text-fg'}`,
                  )}
                >
                  {copy.action}
                </Link>
              )}
            </li>
          )
        })}
      </ol>
    </section>
  )
}
