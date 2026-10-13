import { isKnownStatus, type KnownBookStatus } from '@/api/status'
import type { BookStatus } from '@/api/types.gen'
import { msg } from '@/lib/messages'

// Lớp Tailwind viết nguyên chuỗi để bộ quét không bỏ sót.
const tone: Record<KnownBookStatus, string> = {
  queued: 'bg-st-idle-bg text-st-idle',
  running: 'bg-st-running-bg text-st-running',
  waiting_review: 'bg-st-review-bg text-st-review',
  paused: 'bg-st-idle-bg text-st-idle',
  budget_exhausted: 'bg-st-budget-bg text-st-budget',
  completed: 'bg-st-done-bg text-st-done',
  failed: 'bg-st-error-bg text-st-error',
  interrupted: 'bg-st-error-bg text-st-error',
  deleting: 'bg-st-idle-bg text-st-idle',
}

export function statusLabel(status: BookStatus): string {
  return isKnownStatus(status) ? msg.status[status] : msg.status.unknown
}

export function StatusBadge({ status }: { status: BookStatus }) {
  const known = isKnownStatus(status)
  return (
    <span
      className={`inline-flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold ${
        known ? tone[status] : 'bg-st-idle-bg text-st-idle'
      }`}
    >
      {status === 'running' && (
        <span aria-hidden className="pulse-dot size-[7px] rounded-full bg-current" />
      )}
      {statusLabel(status)}
    </span>
  )
}
