import { Link, useNavigate } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { errorMessage } from '@/api/client'
import { useChangePassword, useLogout } from '@/api/queries'
import { PageHeader } from '@/components/app-shell'
import { Button } from '@/components/ui/button'
import { Field, FormError } from '@/components/ui/field'
import { msg } from '@/lib/messages'

type Errors = Partial<Record<'current' | 'next' | 'confirm', string>>

export function AccountPage() {
  const change = useChangePassword()
  const logout = useLogout()
  const navigate = useNavigate()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [errors, setErrors] = useState<Errors>({})

  function submit(e: FormEvent) {
    e.preventDefault()
    const errs: Errors = {}
    if (current === '') errs.current = msg.account.currentRequired
    if ([...next].length < 10) errs.next = msg.account.newTooShort
    if (confirm !== next) errs.confirm = msg.account.mismatch
    setErrors(errs)
    if (Object.keys(errs).length > 0) return
    change.mutate(
      { current_password: current, new_password: next },
      {
        onSuccess: () => {
          setCurrent('')
          setNext('')
          setConfirm('')
        },
      },
    )
  }

  return (
    <>
      <PageHeader
        title={msg.account.title}
        back={
          <Link
            to="/settings/keys"
            aria-label={msg.back}
            className="flex size-11 items-center justify-center text-fg"
          >
            <ArrowLeft aria-hidden size={22} />
          </Link>
        }
      />
      <div className="flex flex-col gap-4 px-4 pt-3.5 pb-7">
        <form
          onSubmit={submit}
          noValidate
          aria-labelledby="pw-title"
          className="flex flex-col gap-3.5 rounded-card border border-line bg-surface p-4"
        >
          <h2 id="pw-title" className="m-0 text-base font-semibold">
            {msg.account.changePasswordTitle}
          </h2>
          <Field
            label={msg.account.currentPassword}
            type="password"
            autoComplete="current-password"
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
            error={errors.current}
          />
          <Field
            label={msg.account.newPassword}
            type="password"
            autoComplete="new-password"
            hint={msg.auth.passwordHint}
            value={next}
            onChange={(e) => setNext(e.target.value)}
            error={errors.next}
          />
          <Field
            label={msg.account.confirmPassword}
            type="password"
            autoComplete="new-password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            error={errors.confirm}
          />
          {change.isError && <FormError>{errorMessage(change.error)}</FormError>}
          {change.isSuccess && (
            <p role="status" className="m-0 rounded-control bg-accent-soft px-3.5 py-3 text-sm text-accent">
              {msg.account.changed}
            </p>
          )}
          <Button type="submit" disabled={change.isPending}>
            {change.isPending ? msg.account.submitting : msg.account.submit}
          </Button>
        </form>

        <Button
          variant="secondary"
          className="!text-st-error"
          disabled={logout.isPending}
          onClick={() => logout.mutate(undefined, { onSettled: () => void navigate({ to: '/login' }) })}
        >
          {logout.isPending ? msg.account.loggingOut : msg.account.logout}
        </Button>
      </div>
    </>
  )
}
