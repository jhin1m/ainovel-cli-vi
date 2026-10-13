import { Link } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { errorMessage } from '@/api/client'
import { useSignup } from '@/api/queries'
import { Button, buttonClass } from '@/components/ui/button'
import { Field, FormError } from '@/components/ui/field'
import { msg } from '@/lib/messages'

type Errors = Partial<Record<'email' | 'password' | 'displayName', string>>

export function SignupForm({ onSuccess }: { onSuccess: () => void }) {
  const signup = useSignup()
  const [form, setForm] = useState({ email: '', password: '', displayName: '' })
  const [errors, setErrors] = useState<Errors>({})
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) =>
    setForm((f) => ({ ...f, [k]: e.target.value }))

  function submit(e: FormEvent) {
    e.preventDefault()
    const next: Errors = {}
    if (form.email.trim() === '') next.email = msg.auth.emailRequired
    if ([...form.password].length < 10) next.password = msg.auth.passwordTooShort
    if (form.displayName.trim() === '') next.displayName = msg.auth.displayNameRequired
    setErrors(next)
    if (Object.keys(next).length > 0) return
    signup.mutate(
      {
        email: form.email.trim(),
        password: form.password,
        display_name: form.displayName.trim(),
      },
      { onSuccess },
    )
  }

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-4">
      <Field
        label={msg.auth.displayName}
        autoComplete="name"
        value={form.displayName}
        onChange={set('displayName')}
        error={errors.displayName}
      />
      <Field
        label={msg.auth.email}
        type="email"
        autoComplete="username"
        inputMode="email"
        value={form.email}
        onChange={set('email')}
        error={errors.email}
      />
      <Field
        label={msg.auth.password}
        type="password"
        autoComplete="new-password"
        value={form.password}
        onChange={set('password')}
        hint={msg.auth.passwordHint}
        error={errors.password}
      />
      {signup.isError && <FormError>{errorMessage(signup.error)}</FormError>}
      <Button type="submit" disabled={signup.isPending} className="min-h-[52px]">
        {msg.auth.signupSubmit}
      </Button>
      <p className="m-0 text-center text-sm text-muted">
        {msg.auth.haveAccount}{' '}
        <Link to="/login" className={buttonClass('ghost', '!min-h-[44px] !px-2 text-accent')}>
          {msg.auth.goLogin}
        </Link>
      </p>
    </form>
  )
}
