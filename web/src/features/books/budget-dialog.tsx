import { useState, type FormEvent } from 'react'
import { errorMessage } from '@/api/client'
import { usePatchBook, useResumeBook } from '@/api/queries'
import { RESUMABLE } from '@/api/status'
import type { Book } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { Field, FormError } from '@/components/ui/field'
import { formatUSD } from '@/lib/format'
import { msg } from '@/lib/messages'
import { MAX_BUDGET_USD, parseRaisedBudget } from './new-book-logic'

/**
 * Trạng thái mà sau khi lưu ngân sách thì resume được: server đổi `budget_exhausted → paused`
 * khi nâng ngân sách, các trạng thái còn lại giữ nguyên (books/handlers.go handlePatch).
 */
const CAN_RESUME_AFTER_SAVE = new Set(['budget_exhausted', ...RESUMABLE])

interface Props {
  book: Book
  onClose: () => void
  /** Lưu xong: thông báo kết quả và trạng thái mới cho trang truyện (hộp thoại đã đóng). */
  onSaved?: (notice: string, status: string) => void
}

function Form({ book, onClose, onSaved }: Props) {
  const t = msg.book.budget
  const spent = book.snapshot?.total_cost_usd ?? 0
  const patch = usePatchBook(book.id)
  const resume = useResumeBook(book.id)
  const [value, setValue] = useState(() => String(Math.min(MAX_BUDGET_USD, Math.ceil(book.budget_usd + 5))))
  const [invalid, setInvalid] = useState(false)
  const [resumeError, setResumeError] = useState('')
  // Lần gửi gần nhất có resume không (để đổi nhãn nút đang chạy).
  const [resuming, setResuming] = useState(false)
  const canResume = CAN_RESUME_AFTER_SAVE.has(book.status)
  const busy = patch.isPending || resume.isPending

  const done = (notice: string, status: string) => {
    onSaved?.(notice, status)
    onClose()
  }

  // Dùng mutateAsync: đóng hộp thoại (Esc) giữa chừng thì callback của mutate() không chạy
  // nữa, còn promise vẫn chạy tiếp nên ngân sách đã lưu vẫn được resume và báo cho trang truyện.
  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const v = parseRaisedBudget(value, spent)
    setInvalid(v === null)
    setResumeError('')
    if (v === null) return
    // Nút submit được bấm; Enter trong ô nhập chọn nút submit đầu tiên ("Lưu và viết tiếp").
    const submitter = (e.nativeEvent as SubmitEvent).submitter
    const thenResume = canResume && submitter?.dataset.resume === 'true'
    setResuming(thenResume)
    let b: Book
    try {
      b = await patch.mutateAsync({ budget_usd: v })
    } catch {
      return // lỗi hiện qua patch.error
    }
    if (!thenResume) {
      done(canResume ? t.saved : t.savedPlain, b.status)
      return
    }
    try {
      const r = await resume.mutateAsync()
      done(t.savedResumed, r.book.status)
    } catch (err) {
      // Ngân sách đã lưu; resume lỗi thì báo rõ, giữ hộp thoại để thử lại.
      setResumeError(t.resumeFailed(errorMessage(err)))
    }
  }

  return (
    <form onSubmit={(e) => void submit(e)} className="flex flex-col gap-4" noValidate>
      <p className="m-0 text-sm text-muted">{t.current(formatUSD(spent), formatUSD(book.budget_usd))}</p>
      <Field
        label={t.label}
        inputMode="decimal"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        error={invalid ? t.invalid(MAX_BUDGET_USD) : undefined}
      />
      {patch.isError && <FormError>{errorMessage(patch.error)}</FormError>}
      {resumeError && <FormError>{resumeError}</FormError>}
      <div className="flex flex-wrap justify-end gap-2">
        <Button variant="ghost" disabled={busy} onClick={onClose}>
          {msg.cancel}
        </Button>
        {canResume ? (
          <>
            {/* Nút chính đứng trước trong DOM để Enter chọn nó; hiển thị sau cùng bên phải. */}
            <Button type="submit" data-resume="true" className="order-last" disabled={busy}>
              {resume.isPending ? t.resuming : patch.isPending && resuming ? t.submitting : t.submitResume}
            </Button>
            <Button type="submit" variant="secondary" disabled={busy}>
              {patch.isPending && !resuming ? t.submitting : t.saveOnly}
            </Button>
          </>
        ) : (
          <Button type="submit" disabled={busy}>
            {patch.isPending ? t.submitting : t.submit}
          </Button>
        )}
      </div>
    </form>
  )
}

/** Form được mount lại mỗi lần mở nên không giữ trạng thái cũ. */
export function BudgetDialog({ book, open, onClose, onSaved }: Props & { open: boolean }) {
  return (
    <Dialog open={open} title={msg.book.budget.title} onClose={onClose}>
      <Form book={book} onClose={onClose} onSaved={onSaved} />
    </Dialog>
  )
}
