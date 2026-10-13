import { createFileRoute, Outlet } from '@tanstack/react-router'
import { AppShell } from '@/components/app-shell'
import { ErrorState } from '@/components/ui/states'
import { errorMessage } from '@/api/client'
import { requireUser } from '@/features/auth/guard'
import { msg } from '@/lib/messages'

export const Route = createFileRoute('/_app')({
  beforeLoad: ({ context }) => requireUser(context.queryClient),
  component: () => (
    <AppShell>
      <Outlet />
    </AppShell>
  ),
  errorComponent: ({ error, reset }) => (
    <div className="p-5">
      <ErrorState title={msg.genericError} detail={errorMessage(error)} onRetry={reset} />
    </div>
  ),
})
