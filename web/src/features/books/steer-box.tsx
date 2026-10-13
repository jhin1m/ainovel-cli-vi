import { MessageSquarePlus } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { errorMessage } from '@/api/client'
import { useSteer } from '@/api/queries'
import type { Book } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { inputClass } from '@/components/ui/field'
import { msg } from '@/lib/messages'

const MAX_STEER_CHARS = 2000

interface Props {
  bookId: string
  pending: string
  disabled?: boolean
  /** `inline`: thẻ riêng trong trang (màn rộng). `sheet`: nằm trong hộp thoại góp ý ở mobile, không viền. */
  variant?: 'inline' | 'sheet'
}

export function SteerBox({ bookId, pending, disabled, variant = 'inline' }: Props) {
  const steer = useSteer(bookId)
  const [text, setText] = useState('')
  const [notice, setNotice] = useState('')
  const trimmed = text.trim()
  const tooLong = [...text].length > MAX_STEER_CHARS

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (!trimmed || tooLong) return
    setNotice('')
    steer.mutate(trimmed, {
      onSuccess: (r) => {
        setText('')
        setNotice(r.queued ? msg.book.steer.queued : msg.book.steer.sent)
      },
    })
  }

  return (
    <form
      onSubmit={submit}
      aria-label={msg.book.steer.title}
      className={`flex flex-col gap-2 ${
        variant === 'inline' ? 'rounded-card border border-line-card bg-surface p-4 shadow-card' : ''
      }`}
    >
      {pending && (
        <p className="m-0 text-sm text-muted">
          <span className="font-semibold">{msg.book.steer.pending}:</span> {pending}
        </p>
      )}
      <div className="flex items-end gap-2">
        <textarea
          aria-label={msg.book.steer.title}
          rows={variant === 'sheet' ? 3 : 1}
          value={text}
          disabled={disabled}
          placeholder={msg.book.steer.placeholder}
          onChange={(e) => setText(e.target.value)}
          className={`${inputClass} max-h-32 min-h-[44px] flex-1 resize-y py-2.5`}
        />
        <Button type="submit" disabled={disabled || steer.isPending || !trimmed || tooLong}>
          {steer.isPending ? msg.book.steer.sending : msg.book.steer.submit}
        </Button>
      </div>
      {tooLong && <p role="alert" className="m-0 text-sm text-st-error">{msg.book.steer.tooLong}</p>}
      {steer.isError && <p role="alert" className="m-0 text-sm text-st-error">{errorMessage(steer.error)}</p>}
      {notice && !steer.isError && <p role="status" className="m-0 text-sm text-muted">{notice}</p>}
      <p className="m-0 text-xs text-muted">{msg.book.steer.hint}</p>
    </form>
  )
}

/** Điện thoại (<768px): nút "Góp ý" nổi ở góc dưới, mở sheet chứa ô góp ý. Ẩn từ md. */
export function SteerFab({ book }: { book: Book }) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-label={book.pending_text ? `${msg.book.steer.open} (${msg.book.steer.hasPending})` : undefined}
        className="fixed right-4 bottom-[calc(1rem+env(safe-area-inset-bottom))] z-30 inline-flex min-h-12 cursor-pointer items-center gap-2 rounded-full border-0 bg-accent px-5 text-base font-semibold text-on-accent shadow-lg hover:bg-accent-hover md:hidden"
      >
        <MessageSquarePlus aria-hidden size={20} />
        {msg.book.steer.open}
        {book.pending_text && <span aria-hidden className="size-2 rounded-full bg-warm-bg" />}
      </button>
      <Dialog open={open} title={msg.book.steer.title} onClose={() => setOpen(false)}>
        <div className="flex flex-col gap-3">
          <SteerBox bookId={book.id} pending={book.pending_text} disabled={book.status === 'deleting'} variant="sheet" />
          <Button variant="secondary" onClick={() => setOpen(false)}>
            {msg.book.steer.close}
          </Button>
        </div>
      </Dialog>
    </>
  )
}
