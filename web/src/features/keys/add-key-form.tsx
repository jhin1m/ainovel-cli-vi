import { Link } from '@tanstack/react-router'
import { ExternalLink, Eye, EyeOff } from 'lucide-react'
import { useEffect, useId, useRef, useState, type FormEvent } from 'react'
import { ApiError, errorMessage } from '@/api/client'
import { useCreateKey, useProviders, useTestKey } from '@/api/queries'
import type { APIKey } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Field, FormError, inputClass } from '@/components/ui/field'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { msg } from '@/lib/messages'
import { KeyCheckStatus } from './key-check-status'
import { providerMeta } from './provider-meta'
import { ProviderPicker } from './provider-picker'

type Errors = Partial<Record<'provider' | 'apiKey', string>>

interface Props {
  /** Gọi ngay khi key đã lưu, không chờ kết quả kiểm. */
  onSaved: (key: APIKey) => void
  defaultProvider?: string
  /** Bản gọn để nhúng vào trang khác (vd. trang tạo truyện). */
  compact?: boolean
  /** Có thì hiện nút "Huỷ" (trước khi lưu) và "Xong" (sau khi lưu). */
  onClose?: () => void
}

/**
 * Form thêm key: chọn provider → tên gợi nhớ → API key → lưu. Lưu xong tự kiểm key
 * một lần (theo `key_id`) để bắt lỗi chọn sai provider trước khi có truyện.
 */
export function AddKeyForm({ onSaved, defaultProvider, compact, onClose }: Props) {
  const providers = useProviders()
  const create = useCreateKey()
  const check = useTestKey()
  const formRef = useRef<HTMLFormElement>(null)
  const savedRef = useRef<HTMLParagraphElement>(null)
  const ids = useId()
  const [provider, setProvider] = useState('')
  const [label, setLabel] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [showKey, setShowKey] = useState(false)
  const [errors, setErrors] = useState<Errors>({})
  const [attempt, setAttempt] = useState(0)
  const [saved, setSaved] = useState<APIKey | null>(null)

  // Sau mỗi lần bấm lưu bị lỗi field, đưa focus tới lỗi đầu tiên.
  useEffect(() => {
    if (attempt === 0) return
    const first = formRef.current?.querySelector<HTMLElement>('[aria-invalid="true"]')
    if (first?.getAttribute('role') === 'radiogroup') {
      first.querySelector<HTMLElement>('[tabindex="0"]')?.focus()
    } else {
      first?.focus()
    }
  }, [attempt])

  // Form vừa bị thay bằng khối kết quả: giữ focus trong vùng này (không rơi về body).
  useEffect(() => {
    if (saved) savedRef.current?.focus()
  }, [saved])

  if (providers.isPending) return <LoadingState />
  if (providers.isError) {
    return (
      <ErrorState
        title={msg.keys.noProviders}
        detail={errorMessage(providers.error)}
        onRetry={() => void providers.refetch()}
      />
    )
  }

  const options = providers.data
  const fallback = options.some((p) => p.name === defaultProvider) ? defaultProvider : options[0]?.name
  const chosen = provider || fallback || ''
  const info = options.find((p) => p.name === chosen)
  const meta = providerMeta(chosen)

  if (saved) {
    const name = [info?.label ?? saved.provider, saved.label].filter(Boolean).join(' · ')
    return (
      <div className="flex flex-col gap-3.5">
        <p ref={savedRef} tabIndex={-1} className="m-0 text-sm font-medium">
          {msg.keys.savedNamed(name)}
        </p>
        <KeyCheckStatus pending={check.isPending} result={check.data} error={check.error} />
        {onClose && (
          <Button className="self-end" onClick={onClose}>
            {msg.keys.done}
          </Button>
        )}
      </div>
    )
  }

  function submit(e: FormEvent) {
    e.preventDefault()
    const next: Errors = {}
    if (chosen === '') next.provider = msg.keys.providerRequired
    if (apiKey.trim() === '') next.apiKey = msg.keys.apiKeyRequired
    setErrors(next)
    if (Object.keys(next).length > 0) {
      setAttempt((n) => n + 1)
      return
    }
    // mutateAsync thay cho callback của mutate(): callback đó bị bỏ qua nếu form đã
    // unmount (đóng dialog lúc đang lưu), khi đó key đã lưu mà onSaved không chạy.
    create
      .mutateAsync({ provider: chosen, label: label.trim(), api_key: apiKey.trim() })
      .then(
        (key) => {
          setApiKey('')
          setSaved(key)
          check.mutate(key.id)
          onSaved(key)
        },
        () => {
          // Lỗi đã hiện qua create.error.
        },
      )
  }

  const groupLabel = `${ids}-provider`
  const blurbId = `${ids}-blurb`
  const providerErr = `${ids}-provider-err`
  const keyInput = `${ids}-key`
  const keyErr = `${ids}-key-err`

  return (
    <form ref={formRef} onSubmit={submit} noValidate className={`flex flex-col ${compact ? 'gap-3' : 'gap-4'}`}>
      <div className="flex flex-col gap-2">
        <span id={groupLabel} className="text-sm font-medium">
          {msg.keys.provider}
        </span>
        <ProviderPicker
          providers={options}
          value={chosen}
          onChange={setProvider}
          labelledBy={groupLabel}
          describedBy={[meta ? blurbId : '', errors.provider ? providerErr : ''].filter(Boolean).join(' ') || undefined}
          invalid={Boolean(errors.provider)}
          compact={compact}
        />
        {meta && (
          <p id={blurbId} className="m-0 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted">
            {!compact && <span>{meta.blurb}</span>}
            <a
              href={meta.keyUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex min-h-11 items-center gap-1 font-medium text-accent"
            >
              {msg.keys.getKey(info?.label ?? chosen)}
              <ExternalLink aria-hidden size={14} />
              <span className="sr-only">{msg.keys.newTab}</span>
            </a>
          </p>
        )}
        {errors.provider && (
          <span id={providerErr} role="alert" className="text-sm text-st-error">
            {errors.provider}
          </span>
        )}
      </div>

      <Field
        label={msg.keys.label}
        placeholder={info?.label ?? msg.keys.labelPlaceholder}
        hint={compact ? undefined : msg.keys.labelHint}
        maxLength={60}
        value={label}
        onChange={(e) => setLabel(e.target.value)}
      />

      <div className="flex flex-col gap-1.5">
        <label htmlFor={keyInput} className="text-sm font-medium">
          {msg.keys.apiKey}
        </label>
        <div className="relative">
          <input
            id={keyInput}
            type={showKey ? 'text' : 'password'}
            autoComplete="off"
            spellCheck={false}
            autoCapitalize="off"
            placeholder={msg.keys.apiKeyPlaceholder}
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            aria-invalid={errors.apiKey ? true : undefined}
            aria-describedby={errors.apiKey ? keyErr : undefined}
            className={`${inputClass} pr-12`}
          />
          <button
            type="button"
            aria-label={msg.keys.showKey}
            aria-pressed={showKey}
            onClick={() => setShowKey((v) => !v)}
            className={`absolute inset-y-0 right-0 flex w-11 cursor-pointer items-center justify-center rounded-control border-0 bg-transparent ${
              showKey ? 'text-accent' : 'text-muted'
            }`}
          >
            {showKey ? <EyeOff aria-hidden size={18} /> : <Eye aria-hidden size={18} />}
          </button>
        </div>
        {errors.apiKey && (
          <span id={keyErr} role="alert" className="text-sm text-st-error">
            {errors.apiKey}
          </span>
        )}
      </div>

      {create.isError && (
        <FormError>
          {create.error instanceof ApiError && create.error.code === 'key_limit' ? (
            <>
              {msg.keys.limit}{' '}
              <Link to="/settings/keys" className="font-semibold text-st-error underline">
                {msg.keys.limitLink}
              </Link>
            </>
          ) : (
            errorMessage(create.error)
          )}
        </FormError>
      )}

      <div className="flex gap-2.5">
        {onClose && (
          <Button variant="secondary" className="flex-1" onClick={onClose}>
            {msg.cancel}
          </Button>
        )}
        <Button type="submit" className="flex-1" disabled={create.isPending}>
          {create.isPending ? msg.keys.submitting : msg.keys.submit}
        </Button>
      </div>
    </form>
  )
}
