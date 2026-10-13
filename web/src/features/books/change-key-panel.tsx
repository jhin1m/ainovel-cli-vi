import { Link } from '@tanstack/react-router'
import { useId, useState } from 'react'
import { errorMessage } from '@/api/client'
import { RESUMABLE } from '@/api/status'
import { useKeys, usePatchBook, useProviders, useResumeBook } from '@/api/queries'
import type { APIKey, Book } from '@/api/types.gen'
import { ProviderIcon } from '@/components/provider-icon'
import { Button } from '@/components/ui/button'
import { FormError, inputClass } from '@/components/ui/field'
import { msg } from '@/lib/messages'


const keyName = (k: APIKey, providerLabel: string) =>
  `${[providerLabel, k.label !== providerLabel ? k.label : ''].filter(Boolean).join(' · ')} ••••${k.last4}`

interface Props {
  book: Book
  /** Gọi sau khi đổi key xong (vd. đóng hộp thoại), kèm thông báo và trạng thái mới của truyện. */
  onDone?: (notice: string, status: string) => void
  /** Key đã đổi nhưng resume lỗi; dùng khi panel có thể đã biến mất (banner). */
  onResumeError?: (text: string, status: string) => void
}

/**
 * Đổi key của truyện. Chỉ liệt kê key cùng nhà cung cấp: server chặn key khác provider
 * (`provider_mismatch`). Luôn hiện key đang dùng để user nhận ra key lạ.
 */
export function ChangeKeyPanel({ book, onDone, onResumeError }: Props) {
  const keys = useKeys()
  const providers = useProviders()
  const patch = usePatchBook(book.id)
  const resume = useResumeBook(book.id)
  const id = useId()
  const [picked, setPicked] = useState('')
  const [notice, setNotice] = useState('')
  const [resumeError, setResumeError] = useState('')
  const t = msg.book.changeKey

  const providerLabel = providers.data?.find((p) => p.name === book.provider)?.label ?? book.provider
  const all = keys.data ?? []
  const current = all.find((k) => k.id === book.api_key_id)
  const others = all.filter((k) => k.provider === book.provider && k.id !== book.api_key_id)
  const choice = others.find((k) => k.id === picked) ?? others[0]
  const busy = patch.isPending || resume.isPending
  const canResume = RESUMABLE.has(book.status)

  // Dùng mutateAsync: trong banner "key đã bị xoá", PATCH xong là banner (và panel này) biến mất,
  // callback của mutate() sẽ không chạy nữa nên kết quả resume phải đi qua promise.
  const apply = async (andResume: boolean) => {
    if (!choice) return
    setNotice('')
    setResumeError('')
    let b: Book
    try {
      b = await patch.mutateAsync({ api_key_id: choice.id })
    } catch {
      return // lỗi hiện qua patch.error
    }
    // Có `onDone` thì nơi gọi hiện thông báo; hiện cả ở đây sẽ bị đọc hai lần.
    const report = (text: string, status: string) => (onDone ? onDone(text, status) : setNotice(text))
    if (!andResume) {
      report(t.saved, b.status)
      return
    }
    try {
      const r = await resume.mutateAsync()
      report(t.savedResumed, r.book.status)
    } catch (e) {
      const text = t.resumeFailed(errorMessage(e))
      setResumeError(text)
      onResumeError?.(text, b.status)
    }
  }

  return (
    <div className="flex flex-col gap-3 text-fg">
      <p className="m-0 flex items-center gap-2 text-sm">
        <ProviderIcon name={book.provider} size={18} />
        <span className="min-w-0 break-words">
          {current ? t.current(keyName(current, providerLabel)) : keys.isPending ? '' : t.currentMissing}
        </span>
      </p>

      {keys.isError && <FormError>{errorMessage(keys.error)}</FormError>}

      {keys.isSuccess && others.length === 0 && (
        <p className="m-0 flex flex-wrap items-center gap-x-2 text-sm">
          <span>{t.none(providerLabel)}</span>
          <Link to="/settings/keys" className="inline-flex min-h-11 items-center font-medium">
            {t.addKey(providerLabel)}
          </Link>
        </p>
      )}

      {choice && (
        <>
          <div className="flex max-w-md flex-col gap-1.5">
            <label htmlFor={id} className="text-sm font-medium">
              {t.pick}
            </label>
            <div className="relative">
              <ProviderIcon
                name={book.provider}
                size={20}
                className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2"
              />
              <select
                id={id}
                className={`${inputClass} pl-11`}
                value={choice.id}
                disabled={busy}
                onChange={(e) => setPicked(e.target.value)}
              >
                {others.map((k) => (
                  <option key={k.id} value={k.id}>
                    {keyName(k, providerLabel)}
                  </option>
                ))}
              </select>
            </div>
            <p className="m-0 text-xs text-muted">{t.sameProvider}</p>
          </div>
          <div className="flex flex-wrap gap-2">
            {canResume && (
              <Button disabled={busy} onClick={() => void apply(true)}>
                {busy ? t.saving : t.useResume}
              </Button>
            )}
            <Button variant={canResume ? 'secondary' : 'primary'} disabled={busy} onClick={() => void apply(false)}>
              {t.use}
            </Button>
          </div>
        </>
      )}

      {patch.isError && <FormError>{errorMessage(patch.error)}</FormError>}
      {resumeError && <FormError>{resumeError}</FormError>}
      {notice && (
        <p role="status" className="m-0 text-sm font-medium">
          {notice}
        </p>
      )}
    </div>
  )
}
