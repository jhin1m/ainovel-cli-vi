import { useState, type FormEvent } from 'react'
import { errorMessage } from '@/api/client'
import { useSteer } from '@/api/queries'
import { Button } from '@/components/ui/button'
import { inputClass } from '@/components/ui/field'
import { msg } from '@/lib/messages'

const MAX_STEER_CHARS = 2000

export function SteerBox({ bookId, pending, disabled }: { bookId: string; pending: string; disabled?: boolean }) {
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
      className="pb-safe sticky bottom-0 z-10 flex flex-col gap-2 border-t border-line bg-surface px-4 pt-3 lg:rounded-card lg:border"
    >
      {pending && (
        <p className="m-0 text-[13px] text-muted">
          <span className="font-semibold">{msg.book.steer.pending}:</span> {pending}
        </p>
      )}
      <div className="flex items-end gap-2">
        <textarea
          aria-label={msg.book.steer.title}
          rows={1}
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
      {tooLong && <p role="alert" className="m-0 text-[13px] text-st-error">{msg.book.steer.tooLong}</p>}
      {steer.isError && <p role="alert" className="m-0 text-[13px] text-st-error">{errorMessage(steer.error)}</p>}
      {notice && !steer.isError && <p role="status" className="m-0 text-[13px] text-muted">{notice}</p>}
      <p className="m-0 pb-1 text-[12px] text-muted">{msg.book.steer.hint}</p>
    </form>
  )
}
