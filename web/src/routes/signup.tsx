import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { AuthLayout } from '@/features/auth/auth-layout'
import { redirectIfSignedIn } from '@/features/auth/guard'
import { SignupForm } from '@/features/auth/signup-form'
import { msg } from '@/lib/messages'

export const Route = createFileRoute('/signup')({
  beforeLoad: ({ context }) => redirectIfSignedIn(context.queryClient),
  component: SignupPage,
})

function SignupPage() {
  const navigate = useNavigate()
  return (
    <AuthLayout title={msg.auth.signupTitle} subtitle={msg.auth.signupSubtitle}>
      <SignupForm onSuccess={() => void navigate({ to: '/' })} />
    </AuthLayout>
  )
}
