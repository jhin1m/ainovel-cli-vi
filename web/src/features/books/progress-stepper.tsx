import { Check } from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import type { AgentSnapshot, Book, BookSnapshot } from '@/api/types.gen'
import { agentMeta } from '@/features/book-stream/agent-meta'
import { msg } from '@/lib/messages'

/** Số bước của stepper; `index === STEP_COUNT` nghĩa là truyện đã xong (mọi bước đã qua). */
export const STEP_COUNT = 4

export interface Step {
  /** 0 lập dàn ý, 1 viết chương, 2 biên tập, 3 chờ bạn duyệt, 4 đã xong. */
  index: number
  /** Chương gắn với bước hiện tại (0 nếu không áp dụng). */
  chapter: number
}

/**
 * Agent đang chạy trong snapshot. Host đặt `state = "working"` khi engine giao việc và
 * "idle" khi xong (internal/host/observer.go); mọi state khác "idle"/rỗng đều coi là đang chạy.
 * Thứ tự agents từ Go map không cố định nên sắp theo tên để kết quả ổn định.
 */
export function activeAgent(s: BookSnapshot | undefined | null): AgentSnapshot | undefined {
  return [...(s?.agents ?? [])]
    .filter((a) => a.state !== '' && a.state !== 'idle')
    .sort((a, b) => a.name.localeCompare(b.name))[0]
}

const EDIT_FLOWS = new Set(['reviewing', 'rewriting', 'polishing'])

/**
 * Bước hiện tại, map từ code worker/host:
 * - truyện `completed` hoặc phase `complete` → đã xong;
 * - `waiting_review` (DB) → chờ bạn duyệt chương vừa xong;
 * - phase `init`/`premise`/`outline` (hoặc chưa có snapshot) → lập dàn ý;
 * - phase `writing`: flow `reviewing`/`rewriting`/`polishing` → biên tập, còn lại → viết chương.
 *   Khi truyện đang chạy, agent đang làm việc được ưu tiên: architect (lập cung/quyển kế tiếp
 *   ở chế độ dài) → lập dàn ý, editor → biên tập.
 * Truyện dừng/lỗi giữ bước nơi nó dừng lại.
 */
export function stepFromSnapshot(s: BookSnapshot | undefined | null, status: string): Step {
  const done = s?.completed_count ?? 0
  if (status === 'completed' || s?.phase === 'complete') return { index: STEP_COUNT, chapter: done }
  if (status === 'waiting_review') return { index: 3, chapter: done }
  if (!s) return { index: 0, chapter: 0 }

  if (s.phase !== 'writing') return { index: 0, chapter: 0 }
  const agent = status === 'running' ? activeAgent(s)?.name.toLowerCase() ?? '' : ''
  if (agent.includes('architect')) return { index: 0, chapter: 0 }

  const clamp = (n: number) => (s.total_chapters > 0 ? Math.min(n, s.total_chapters) : n)
  if (agent.includes('editor') || EDIT_FLOWS.has(s.flow)) {
    // Editor duyệt chương vừa commit: lúc đó in_progress_chapter đã về 0 và completed_count = N
    // (internal/store/progress.go), nên chương đang biên tập là N chứ không phải N+1.
    return { index: 2, chapter: clamp(s.in_progress_chapter > 0 ? s.in_progress_chapter : Math.max(done, 1)) }
  }
  return { index: 1, chapter: clamp(s.in_progress_chapter > 0 ? s.in_progress_chapter : done + 1) }
}

/**
 * Mô tả việc agent đang làm cho người đọc. Summary của host có dạng "engine → writer (dòng đầu
 * nhiệm vụ)" khi giao việc và "writer → tên_tool" khi gọi tool (internal/host/observer*.go).
 */
export function agentActivity(a: AgentSnapshot): string {
  const t = msg.book.stepper
  const summary = a.summary.trim()
  const arrow = summary.indexOf('→')
  if (arrow < 0) return summary || t.tools[a.tool] || a.tool
  const from = summary.slice(0, arrow).trim()
  const rest = summary.slice(arrow + 1).trim()
  if (from === 'engine') {
    const task = /\((.+)\)\s*$/.exec(rest)?.[1]?.trim()
    return task || t.working
  }
  return t.tools[rest] ?? rest
}

/**
 * Nhãn các bước. Chế độ tự động không dừng chờ duyệt nên bỏ bước "Chờ bạn duyệt", trừ khi
 * truyện đang chờ duyệt thật (vd. vừa đổi sang tự động lúc đang chờ) để luôn có bước hiện tại.
 */
function stepLabels(chapter: number, withReview: boolean): string[] {
  const t = msg.book.stepper.steps
  const labels = [t.outline, t.write(chapter), t.edit]
  return withReview ? [...labels, t.review] : labels
}

/** Giá trị chỉ được cập nhật tối đa một lần mỗi `ms`; giá trị cuối cùng luôn được áp dụng. */
export function useThrottled<T>(value: T, ms: number): T {
  const [shown, setShown] = useState(value)
  const last = useRef(0)

  useEffect(() => {
    if (Object.is(value, shown)) return
    const wait = Math.max(0, last.current + ms - Date.now())
    const t = setTimeout(() => {
      last.current = Date.now()
      setShown(value)
    }, wait)
    return () => clearTimeout(t)
  }, [value, shown, ms])

  return shown
}

/**
 * Một dòng "agent: việc đang làm". `aria-live` đọc bản đã throttle để không đọc từng cập nhật;
 * vùng live luôn được mount (sr-only là absolute nên không chiếm ô lưới).
 */
function AgentLine({ agent }: { agent: AgentSnapshot | undefined }) {
  const meta = agent ? agentMeta(agent.name) : null
  const activity = agent ? agentActivity(agent) : ''
  const spoken = useThrottled(meta ? msg.book.stepper.agentLine(meta.label, activity) : '', 4000)
  return (
    <>
      {/* Dòng nhìn thấy ẩn khỏi screen reader: vùng live bên dưới đọc cùng nội dung (đã throttle). */}
      {meta && (
        <p aria-hidden className="col-span-full m-0 flex min-w-0 items-center gap-2 text-sm">
          <span className={`h-6 shrink-0 rounded-full px-2.5 text-xs leading-6 font-semibold ${meta.tone}`}>{meta.label}</span>
          <span className="truncate text-muted">{activity}</span>
        </p>
      )}
      <span aria-live="polite" className="sr-only">
        {spoken}
      </span>
    </>
  )
}

interface Props {
  book: Book
  /** Đặt cạnh tiêu đề "Đang làm" (vd. công tắc duyệt từng chương). */
  aside?: ReactNode
}

/** Dải "Đang làm": stepper 4 bước + dòng tóm tắt của agent đang chạy. */
export function ProgressStepper({ book, aside }: Props) {
  const step = stepFromSnapshot(book.snapshot, book.status)
  const labels = stepLabels(step.chapter, book.advance_mode !== 'auto' || book.status === 'waiting_review')
  const agent = book.status === 'running' ? activeAgent(book.snapshot) : undefined

  return (
    <section
      aria-labelledby="progress-title"
      // Điện thoại: tiêu đề, aside, các bước xếp dọc. Từ 640px tiêu đề + aside một hàng, bước ở hàng dưới.
      // Từ 1280px gom cả ba vào một hàng để chừa chiều cao cho hai cột của tab "Đang viết".
      className="grid grid-cols-1 items-start gap-x-6 gap-y-3 rounded-card border border-line-card bg-surface p-4 shadow-card sm:grid-cols-[minmax(0,1fr)_auto] xl:grid-cols-[auto_minmax(0,1fr)_auto] xl:items-center"
    >
      <h2 id="progress-title" className="m-0 text-sm font-semibold text-muted">
        {msg.book.stepper.title}
      </h2>
      {aside && <div className="sm:col-start-2 sm:row-start-1 xl:col-start-3">{aside}</div>}
      <ol
        className={`col-span-full m-0 grid list-none gap-2 p-0 xl:col-span-1 xl:col-start-2 xl:row-start-1 ${
          labels.length === 4 ? 'grid-cols-4' : 'grid-cols-3'
        }`}
      >
        {labels.map((label, i) => {
          const past = i < step.index
          const current = i === step.index
          return (
            <li
              key={i}
              aria-current={current ? 'step' : undefined}
              className="flex min-w-0 flex-col items-start gap-1.5 sm:flex-row sm:items-center"
            >
              <span
                aria-hidden
                className={`flex size-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold tabular-nums ${
                  current
                    ? 'bg-accent text-on-accent'
                    : past
                      ? 'bg-st-done-bg text-st-done'
                      : 'bg-track text-muted'
                }`}
              >
                {past ? <Check size={14} /> : i + 1}
              </span>
              <span className={`text-xs leading-snug sm:text-sm ${current ? 'font-semibold text-fg' : 'text-muted'}`}>
                {label}
                {past && <span className="sr-only"> {msg.book.stepper.stepDone}</span>}
              </span>
            </li>
          )
        })}
      </ol>
      {step.index === STEP_COUNT && <p className="col-span-full m-0 text-sm text-muted">{msg.book.stepper.allDone}</p>}
      <AgentLine agent={agent} />
    </section>
  )
}
