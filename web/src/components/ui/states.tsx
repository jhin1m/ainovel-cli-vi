import { Loader2 } from 'lucide-react'
import type { ReactNode } from 'react'
import { msg } from '@/lib/messages'
import { Button } from './button'

export function Spinner({ className = '' }: { className?: string }) {
  return <Loader2 aria-hidden className={`animate-spin ${className}`} size={18} />
}

export function LoadingState({ label = msg.loading }: { label?: string }) {
  return (
    <div role="status" className="flex items-center justify-center gap-2 py-12 text-muted">
      <Spinner />
      <span>{label}</span>
    </div>
  )
}

export function ErrorState({
  title,
  detail,
  onRetry,
}: {
  title: string
  detail?: string
  onRetry?: () => void
}) {
  return (
    <div
      role="alert"
      className="flex flex-col items-center gap-3 rounded-card border border-line bg-surface p-6 text-center"
    >
      <p className="m-0 font-semibold text-st-error">{title}</p>
      {detail && <p className="m-0 text-sm text-muted">{detail}</p>}
      {onRetry && (
        <Button variant="secondary" onClick={onRetry}>
          {msg.retry}
        </Button>
      )}
    </div>
  )
}

export function EmptyState({
  title,
  hint,
  action,
}: {
  title: string
  hint?: string
  action?: ReactNode
}) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-card border border-dashed border-line-input bg-surface p-8 text-center">
      <p className="m-0 font-serif text-lg font-semibold">{title}</p>
      {hint && <p className="m-0 text-sm text-muted">{hint}</p>}
      {action}
    </div>
  )
}
