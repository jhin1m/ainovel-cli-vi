import { useState, type FormEvent } from 'react'
import { errorMessage } from '@/api/client'
import { usePatchBook } from '@/api/queries'
import type { Book } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { Field, FormError } from '@/components/ui/field'
import { formatUSD } from '@/lib/format'
import { msg } from '@/lib/messages'

const MAX_BUDGET_USD = 50

export function parseBudget(raw: string, spent: number): number | null {
  const v = Number(raw.replace(',', '.'))
  if (!Number.isFinite(v) || v <= spent || v > MAX_BUDGET_USD) return null
  return v
}

function Form({ book, onClose }: { book: Book; onClose: () => void }) {
  const spent = book.snapshot?.total_cost_usd ?? 0
  const patch = usePatchBook(book.id)
  const [value, setValue] = useState(() => String(Math.min(MAX_BUDGET_USD, Math.ceil(book.budget_usd + 5))))
  const [invalid, setInvalid] = useState(false)

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const v = parseBudget(value, spent)
    setInvalid(v === null)
    if (v === null) return
    patch.mutate({ budget_usd: v }, { onSuccess: onClose })
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4" noValidate>
      <p className="m-0 text-sm text-muted">
        {msg.book.budget.current(formatUSD(spent), formatUSD(book.budget_usd))}
      </p>
      <Field
        label={msg.book.budget.label}
        inputMode="decimal"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        error={invalid ? msg.book.budget.invalid : undefined}
      />
      {patch.isError && <FormError>{errorMessage(patch.error)}</FormError>}
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>
          {msg.cancel}
        </Button>
        <Button type="submit" disabled={patch.isPending}>
          {patch.isPending ? msg.book.budget.submitting : msg.book.budget.submit}
        </Button>
      </div>
    </form>
  )
}

/** Form được mount lại mỗi lần mở nên không giữ trạng thái cũ. */
export function BudgetDialog({ book, open, onClose }: { book: Book; open: boolean; onClose: () => void }) {
  return (
    <Dialog open={open} title={msg.book.budget.title} onClose={onClose}>
      <Form book={book} onClose={onClose} />
    </Dialog>
  )
}
