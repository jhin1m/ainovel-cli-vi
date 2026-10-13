import { createRootRouteWithContext, Link, Outlet } from '@tanstack/react-router'
import type { RouterContext } from '@/router'
import { buttonClass } from '@/components/ui/button'
import { msg } from '@/lib/messages'

export const Route = createRootRouteWithContext<RouterContext>()({
  component: () => <Outlet />,
  notFoundComponent: () => (
    <main className="mx-auto flex min-h-dvh max-w-md flex-col items-center justify-center gap-4 px-5 text-center">
      <p className="m-0 font-serif text-xl font-semibold">{msg.notFound}</p>
      <Link to="/" className={buttonClass('primary', 'text-on-accent no-underline hover:text-on-accent')}>
        {msg.nav.library}
      </Link>
    </main>
  ),
})
