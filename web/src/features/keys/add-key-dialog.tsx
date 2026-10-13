import { useState, type FormEvent } from 'react'
import { errorMessage } from '@/api/client'
import { useCreateKey, useProviders } from '@/api/queries'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { Field, FormError, inputClass } from '@/components/ui/field'
import { msg } from '@/lib/messages'

type Errors = Partial<Record<'provider' | 'apiKey' | 'password', string>>

function AddKeyForm({ onDone }: { onDone: () => void }) {
  const providers = useProviders()
  const create = useCreateKey()
  const [provider, setProvider] = useState('')
  const [label, setLabel] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [password, setPassword] = useState('')
  const [errors, setErrors] = useState<Errors>({})

  const options = providers.data ?? []
  const chosen = provider || options[0]?.name || ''

  function submit(e: FormEvent) {
    e.preventDefault()
    const next: Errors = {}
    if (chosen === '') next.provider = msg.keys.providerRequired
    if (apiKey.trim() === '') next.apiKey = msg.keys.apiKeyRequired
    if (password === '') next.password = msg.keys.passwordRequired
    setErrors(next)
    if (Object.keys(next).length > 0) return
    create.mutate(
      { provider: chosen, label: label.trim(), api_key: apiKey.trim(), current_password: password },
      { onSuccess: onDone },
    )
  }

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-3.5">
      <div className="flex flex-col gap-1.5">
        <label htmlFor="key-provider" className="text-sm font-medium">
          {msg.keys.provider}
        </label>
        <select
          id="key-provider"
          className={inputClass}
          value={chosen}
          onChange={(e) => setProvider(e.target.value)}
          aria-invalid={errors.provider ? true : undefined}
        >
          {options.map((p) => (
            <option key={p.name} value={p.name}>
              {p.label}
            </option>
          ))}
        </select>
        {errors.provider && (
          <span role="alert" className="text-[13px] text-st-error">
            {errors.provider}
          </span>
        )}
      </div>
      <Field
        label={msg.keys.label}
        placeholder={msg.keys.labelPlaceholder}
        maxLength={60}
        value={label}
        onChange={(e) => setLabel(e.target.value)}
      />
      <Field
        label={msg.keys.apiKey}
        type="password"
        autoComplete="off"
        placeholder={msg.keys.apiKeyPlaceholder}
        value={apiKey}
        onChange={(e) => setApiKey(e.target.value)}
        error={errors.apiKey}
      />
      <Field
        label={msg.keys.currentPassword}
        type="password"
        autoComplete="current-password"
        hint={msg.keys.currentPasswordHint}
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        error={errors.password}
      />
      {create.isError && <FormError>{errorMessage(create.error)}</FormError>}
      <div className="flex gap-2.5">
        <Button variant="secondary" className="flex-1" onClick={onDone}>
          {msg.cancel}
        </Button>
        <Button type="submit" className="flex-1" disabled={create.isPending}>
          {create.isPending ? msg.keys.submitting : msg.keys.submit}
        </Button>
      </div>
    </form>
  )
}

export function AddKeyDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  return (
    <Dialog open={open} title={msg.keys.addTitle} onClose={onClose}>
      {/* Mount lại mỗi lần mở để xoá sạch mật khẩu/key đã nhập. */}
      <AddKeyForm onDone={onClose} />
    </Dialog>
  )
}
