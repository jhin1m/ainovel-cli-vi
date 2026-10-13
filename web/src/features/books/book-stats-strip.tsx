import { useEffect, useRef, useState, type ReactNode, type RefObject } from 'react'
import type { Book } from '@/api/types.gen'
import { ProgressRing } from '@/components/progress-ring'
import { ProviderIcon } from '@/components/provider-icon'
import { formatNumber, formatUSD, formatUSDPlain } from '@/lib/format'
import { msg } from '@/lib/messages'
import { shortModel } from './book-card'

function Stat({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex shrink-0 flex-col gap-0.5">
      <dt className="text-xs text-muted">{label}</dt>
      <dd className="m-0 flex items-center gap-1.5 text-base font-semibold tabular-nums">{children}</dd>
    </div>
  )
}

interface Props {
  book: Book
  /** Mở hộp thoại ngân sách. Không truyền với truyện thuê bao. */
  onBudget?: () => void
}

/**
 * Dải số liệu dưới tiêu đề: tiến độ, số chữ, model, token vào/ra, chi phí.
 * Truyện thuê bao không có USD và nút ngân sách. Mobile là một dòng cuộn ngang trong dải,
 * không làm trang tràn ngang.
 */
export function BookStatsStrip({ book, onBudget }: Props) {
  const s = book.snapshot
  const done = s?.completed_count ?? 0
  const total = s?.total_chapters ?? 0
  const pct = total > 0 ? (done / total) * 100 : 0
  const model = shortModel(s?.model_name || book.model)
  const provider = s?.provider || book.provider
  const stripRef = useRef<HTMLDivElement>(null)
  const scrolls = useScrollsX(stripRef)

  return (
    <div
      ref={stripRef}
      role="region"
      aria-label={msg.book.stats.label}
      tabIndex={scrolls ? 0 : undefined}
      className="-mx-4 overflow-x-auto overflow-y-hidden px-4 md:mx-0 md:overflow-visible md:px-0"
    >
      <dl className="m-0 flex w-max items-end gap-x-6 gap-y-3 md:w-auto md:flex-wrap">
        <Stat label={msg.book.stats.progress}>
          <ProgressRing value={pct} size={32} stroke={3} />
          {msg.book.stats.chapters(done, total)}
        </Stat>
        <Stat label={msg.book.stats.words}>{formatNumber(s?.total_word_count ?? 0)}</Stat>
        <Stat label={msg.book.stats.model}>
          <ProviderIcon name={provider} size={16} />
          <span className="sr-only">{msg.book.stats.providerModel(provider, model)}</span>
          <span aria-hidden className="max-w-[16rem] truncate">
            {model}
          </span>
        </Stat>
        <Stat label={msg.book.stats.tokens}>
          {formatNumber(s?.total_input_tokens ?? 0)} / {formatNumber(s?.total_output_tokens ?? 0)}
        </Stat>
        {!book.subscription && (
          <Stat label={msg.book.stats.cost}>
            {`${formatUSDPlain(s?.total_cost_usd ?? 0)} / ${formatUSD(book.budget_usd, book.budget_usd % 1 === 0 ? 0 : 2)}`}
            {onBudget && (
              <button
                type="button"
                onClick={onBudget}
                className="-my-2 ml-1 min-h-[44px] cursor-pointer rounded-control border-0 bg-transparent px-2 text-sm font-semibold text-accent hover:bg-track"
              >
                {msg.book.budget.title}
              </button>
            )}
          </Stat>
        )}
      </dl>
    </div>
  )
}

/**
 * Báo dải có đang tràn ngang (cuộn được) không. Chỉ khi đó dải mới cần nhận focus Tab
 * để người dùng bàn phím cuộn được; ở màn rộng dải không cuộn nên bỏ khỏi thứ tự Tab.
 */
function useScrollsX(ref: RefObject<HTMLElement | null>): boolean {
  const [active, setActive] = useState(false)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    const check = () => setActive(el.scrollWidth > el.clientWidth)
    check()
    window.addEventListener('resize', check)
    // Nội dung đổi (số liệu tăng, model dài) cũng làm dải tràn hoặc hết tràn.
    const ro = typeof ResizeObserver === 'function' ? new ResizeObserver(check) : null
    ro?.observe(el)
    if (el.firstElementChild) ro?.observe(el.firstElementChild)
    return () => {
      window.removeEventListener('resize', check)
      ro?.disconnect()
    }
  }, [ref])

  return active
}
