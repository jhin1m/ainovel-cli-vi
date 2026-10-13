import { Link } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { errorMessage } from '@/api/client'
import { useLogin } from '@/api/queries'
import { Button, buttonClass } from '@/components/ui/button'
import { Field, FormError } from '@/components/ui/field'
import { msg } from '@/lib/messages'

export function LoginForm({ onSuccess }: { onSuccess: () => void }) {
  const login = useLogin()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [errors, setErrors] = useState<{ email?: string; password?: string }>({})

  function submit(e: FormEvent) {
    e.preventDefault()
    const next: typeof errors = {}
    if (email.trim() === '') next.email = msg.auth.emailRequired
    if (password === '') next.password = msg.auth.passwordRequired
    setErrors(next)
    if (next.email || next.password) return
    login.mutate({ email: email.trim(), password }, { onSuccess })
  }

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-4">
      <Field
        label={msg.auth.email}
        type="email"
        autoComplete="username"
        inputMode="email"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        error={errors.email}
      />
      <Field
        label={msg.auth.password}
        type="password"
        autoComplete="current-password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        error={errors.password}
      />
      {login.isError && <FormError>{errorMessage(login.error)}</FormError>}
      <Button type="submit" disabled={login.isPending} className="min-h-[52px]">
        {msg.auth.loginSubmit}
      </Button>
      <p className="m-0 text-center text-sm text-muted">
        {msg.auth.noAccount}{' '}
        <Link to="/signup" className={buttonClass('ghost', '!min-h-[44px] !px-2 text-accent')}>
          {msg.auth.goSignup}
        </Link>
      </p>
    </form>
  )
}
