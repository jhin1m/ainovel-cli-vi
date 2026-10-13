import { Link, useNavigate } from '@tanstack/react-router'
import { ChevronRight, Info, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { errorMessage } from '@/api/client'
import { useDeleteKey, useKeys, useLogout, useMe, useProviders } from '@/api/queries'
import type { APIKey } from '@/api/types.gen'
import { PageHeader } from '@/components/app-shell'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { FormError } from '@/components/ui/field'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { formatDate } from '@/lib/format'
import { msg } from '@/lib/messages'
import { AddKeyDialog } from './add-key-dialog'

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
  const keyName = (k: APIKey) => [providerLabel(k.provider), k.label].filter(Boolean).join(' · ')

  function confirmDelete() {
    if (!removing) return
    del.mutate(removing.id, { onSuccess: () => setRemoving(null) })
  }

  return (
    <>
      <PageHeader title={msg.keys.title} />
      <div className="flex flex-col gap-4 px-4 pt-3.5 pb-7">
        <section aria-labelledby="keys-title" className="flex flex-col gap-2.5">
          <h2 id="keys-title" className="m-0 text-[13px] font-semibold tracking-wider text-muted uppercase">
            {msg.keys.section}
          </h2>
          <div className="flex gap-2.5 rounded-control bg-accent-soft p-3 text-[13px] leading-relaxed text-accent">
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
            <p className="m-0 rounded-card border border-line bg-surface p-4 text-sm text-muted">
              {msg.keys.empty}
            </p>
          ) : (
            <ul className="m-0 list-none overflow-hidden rounded-card border border-line bg-surface p-0">
              {keys.data.map((k) => (
                <li
                  key={k.id}
                  className="flex items-center gap-3 border-b border-line py-3.5 pr-2 pl-4 last:border-b-0"
                >
                  <span
                    aria-hidden
                    className="flex size-10 shrink-0 items-center justify-center rounded-[10px] bg-bg text-[13px] font-bold"
                  >
                    {providerLabel(k.provider).slice(0, 2).toUpperCase()}
                  </span>
                  <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="truncate text-[15px] font-semibold">{keyName(k)}</span>
                    <span className="text-[13px] text-muted">
                      ••••{k.last4} · {msg.keys.added(formatDate(k.created_at))}
                    </span>
                  </div>
                  <button
                    type="button"
                    aria-label={msg.keys.remove(keyName(k))}
                    onClick={() => {
                      del.reset()
                      setRemoving(k)
                    }}
                    className="flex size-11 cursor-pointer items-center justify-center border-0 bg-transparent text-st-error"
                  >
                    <Trash2 aria-hidden size={20} />
                  </button>
                </li>
              ))}
            </ul>
          )}

          <Button onClick={() => setAdding(true)} className="self-start">
            <Plus aria-hidden size={18} />
            {msg.keys.addTitle}
          </Button>
        </section>

        <section aria-labelledby="acc-title" className="flex flex-col gap-2.5">
          <h2 id="acc-title" className="m-0 text-[13px] font-semibold tracking-wider text-muted uppercase">
            {msg.keys.accountSection}
          </h2>
          <div className="overflow-hidden rounded-card border border-line bg-surface">
            <div className="flex items-center justify-between gap-3 border-b border-line px-4 py-3.5">
              <span className="text-[15px]">{msg.keys.emailLabel}</span>
              <span className="truncate text-sm text-muted">{me.data?.email ?? ''}</span>
            </div>
            <Link
              to="/settings/account"
              className="flex min-h-[52px] items-center justify-between border-b border-line px-4 text-[15px] text-fg no-underline hover:text-fg"
            >
              {msg.keys.changePassword}
              <ChevronRight aria-hidden size={18} className="text-muted" />
            </Link>
            <button
              type="button"
              disabled={logout.isPending}
              onClick={() => logout.mutate(undefined, { onSettled: () => void navigate({ to: '/login' }) })}
              className="min-h-[52px] w-full cursor-pointer border-0 bg-transparent px-4 text-left text-[15px] font-semibold text-st-error"
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
