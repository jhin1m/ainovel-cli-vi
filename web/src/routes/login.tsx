import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { AuthLayout } from '@/features/auth/auth-layout'
import { LoginForm } from '@/features/auth/login-form'
import { redirectIfSignedIn } from '@/features/auth/guard'
import { msg } from '@/lib/messages'

export const Route = createFileRoute('/login')({
  beforeLoad: ({ context }) => redirectIfSignedIn(context.queryClient),
  component: LoginPage,
})

function LoginPage() {
  const navigate = useNavigate()
  return (
    <AuthLayout title={msg.auth.loginTitle} subtitle={msg.auth.loginSubtitle}>
      <LoginForm onSuccess={() => void navigate({ to: '/' })} />
    </AuthLayout>
  )
}

