import { Link, useNavigate } from '@tanstack/react-router'
import { ChevronRight, Info, KeyRound, Plus, ShieldCheck, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { errorMessage } from '@/api/client'
import { useDeleteKey, useKeys, useLogout, useMe, useProviders, useTestKey } from '@/api/queries'
import type { APIKey } from '@/api/types.gen'
import { PageHeader } from '@/components/app-shell'
import { ProviderIcon } from '@/components/provider-icon'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { FormError } from '@/components/ui/field'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { formatDate } from '@/lib/format'
import { msg } from '@/lib/messages'
import { AddKeyDialog } from './add-key-dialog'
import { KeyCheckStatus } from './key-check-status'

interface KeyCardProps {
  apiKey: APIKey
  providerLabel: string
  name: string
  onRemove: () => void
}

/** Một key đã lưu: icon + nhà cung cấp + tên + 4 ký tự cuối, kèm "Kiểm tra" và xoá. */
function KeyCard({ apiKey: k, providerLabel, name, onRemove }: KeyCardProps) {
  const check = useTestKey()
  const showLabel = k.label !== '' && k.label !== providerLabel
  return (
    <li className="flex flex-col gap-3 rounded-card border border-line-card bg-surface p-4 shadow-card">
      <div className="flex items-start gap-3">
        <span aria-hidden className="flex size-10 shrink-0 items-center justify-center rounded-control bg-bg">
          <ProviderIcon name={k.provider} size={22} />
        </span>
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="truncate text-base font-semibold">{providerLabel}</span>
          {showLabel && <span className="truncate text-sm">{k.label}</span>}
          <span className="text-sm text-muted">
            ••••{k.last4} · {msg.keys.added(formatDate(k.created_at))}
          </span>
        </div>
        <button
          type="button"
          aria-label={msg.keys.remove(name)}
          onClick={onRemove}
          className="-mt-2 -mr-2 flex size-11 shrink-0 cursor-pointer items-center justify-center rounded-control border-0 bg-transparent text-st-error hover:bg-st-error-bg"
        >
          <Trash2 aria-hidden size={20} />
        </button>
      </div>
      <Button
        variant="secondary"
        className="self-start"
        aria-label={msg.keys.checkNamed(name)}
        aria-disabled={check.isPending || undefined}
        onClick={() => {
          if (!check.isPending) check.mutate(k.id)
        }}
      >
        <ShieldCheck aria-hidden size={18} />
        {msg.keys.check}
      </Button>
      <KeyCheckStatus pending={check.isPending} result={check.data} error={check.error} />
    </li>
  )
}

export function KeysPage() {
  const keys = useKeys()
  const providers = useProviders()
  const me = useMe()
  const del = useDeleteKey()
  const logout = useLogout()
  const navigate = useNavigate()
  const [adding, setAdding] = useState(false)
  const [removing, setRemoving] = useState<APIKey | null>(null)

  const providerLabel = (name: string) => providers.data?.find((p) => p.name === name)?.label ?? name
  const keyName = (k: APIKey) => {
    const p = providerLabel(k.provider)
    return k.label && k.label !== p ? `${p} · ${k.label}` : p
  }

  function confirmDelete() {
    if (!removing) return
    del.mutate(removing.id, { onSuccess: () => setRemoving(null) })
  }

  return (
    <>
      <PageHeader title={msg.keys.title} />
      <div className="flex flex-col gap-6 px-4 pt-3.5 pb-7 md:px-0">
        <section aria-labelledby="keys-title" className="flex flex-col gap-3">
          <h2 id="keys-title" className="m-0 text-sm font-semibold tracking-wider text-muted uppercase">
            {msg.keys.section}
          </h2>
          <div className="callout flex max-w-3xl gap-2.5 text-sm leading-relaxed text-accent">
            <Info aria-hidden size={18} className="mt-0.5 shrink-0" />
            <span>{msg.keys.info}</span>
          </div>

          {keys.isPending ? (
            <LoadingState />
          ) : keys.isError ? (
            <ErrorState
              title={msg.keys.loadError}
              detail={errorMessage(keys.error)}
              onRetry={() => void keys.refetch()}
            />
          ) : keys.data.length === 0 ? (
            <div className="flex flex-col items-start gap-3 rounded-card border border-line-card bg-surface p-5 shadow-card">
              <KeyRound aria-hidden size={24} className="text-muted" />
              <p className="m-0 text-sm text-muted">{msg.keys.empty}</p>
              <Button onClick={() => setAdding(true)}>
                <Plus aria-hidden size={18} />
                {msg.keys.addTitle}
              </Button>
            </div>
          ) : (
            <ul className="m-0 grid list-none grid-cols-1 items-start gap-3 p-0 lg:grid-cols-2 2xl:grid-cols-3">
              {keys.data.map((k) => (
                <KeyCard
                  key={k.id}
                  apiKey={k}
                  providerLabel={providerLabel(k.provider)}
                  name={keyName(k)}
                  onRemove={() => {
                    del.reset()
                    setRemoving(k)
                  }}
                />
              ))}
            </ul>
          )}

          {/* Khi tải danh sách lỗi vẫn thêm key được; empty state đã có nút riêng. */}
          {(keys.isError || (keys.isSuccess && keys.data.length > 0)) && (
            <Button onClick={() => setAdding(true)} className="self-start">
              <Plus aria-hidden size={18} />
              {msg.keys.addTitle}
            </Button>
          )}
        </section>

        <section aria-labelledby="acc-title" className="flex flex-col gap-3 lg:max-w-xl">
          <h2 id="acc-title" className="m-0 text-sm font-semibold tracking-wider text-muted uppercase">
            {msg.keys.accountSection}
          </h2>
          <div className="overflow-hidden rounded-card border border-line-card bg-surface shadow-card">
            <div className="flex items-center justify-between gap-3 border-b border-line px-4 py-3.5">
              <span className="text-base">{msg.keys.emailLabel}</span>
              <span className="truncate text-sm text-muted">{me.data?.email ?? ''}</span>
            </div>
            <Link
              to="/settings/account"
              className="flex min-h-13 items-center justify-between border-b border-line px-4 text-base text-fg no-underline hover:text-fg"
            >
              {msg.keys.changePassword}
              <ChevronRight aria-hidden size={18} className="text-muted" />
            </Link>
            <button
              type="button"
              disabled={logout.isPending}
              onClick={() => logout.mutate(undefined, { onSettled: () => void navigate({ to: '/login' }) })}
              className="min-h-13 w-full cursor-pointer border-0 bg-transparent px-4 text-left text-base font-semibold text-st-error"
            >
              {msg.keys.logout}
            </button>
          </div>
        </section>
      </div>

      <AddKeyDialog open={adding} onClose={() => setAdding(false)} />
      <Dialog open={removing !== null} title={msg.keys.confirmTitle} onClose={() => setRemoving(null)}>
        <p className="m-0 mb-4 text-sm leading-relaxed text-muted">
          {removing ? msg.keys.confirmBody(keyName(removing)) : ''}
        </p>
        {del.isError && (
          <div className="mb-3">
            <FormError>{errorMessage(del.error)}</FormError>
          </div>
        )}
        <div className="flex gap-2.5">
          <Button variant="secondary" className="flex-1" onClick={() => setRemoving(null)}>
            {msg.cancel}
          </Button>
          <Button variant="danger" className="flex-1" disabled={del.isPending} onClick={confirmDelete}>
            {msg.delete}
          </Button>
        </div>
      </Dialog>
    </>
  )
}
