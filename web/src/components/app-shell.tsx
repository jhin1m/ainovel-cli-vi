import { Link, useRouterState } from '@tanstack/react-router'
import { BookOpen, Plus, Settings } from 'lucide-react'
import type { ReactNode } from 'react'
import { msg } from '@/lib/messages'

const navItem =
  'flex min-h-[44px] min-w-16 flex-col items-center justify-center gap-0.5 text-xs font-medium text-muted no-underline hover:text-fg'
const sideItem =
  'flex min-h-[44px] items-center gap-3 rounded-control px-3 text-[15px] font-medium text-muted no-underline hover:bg-track hover:text-fg'
const active = { className: `${navItem} !text-fg !font-semibold`, 'aria-current': 'page' as const }
const sideActive = { className: `${sideItem} !bg-track !text-fg !font-semibold`, 'aria-current': 'page' as const }

/** /books/<id> và /books/<id>/read/<n>, trừ /books/new. */
export function routeKind(pathname: string): 'reader' | 'book' | 'other' {
  if (/^\/books\/[^/]+\/read\//.test(pathname)) return 'reader'
  if (/^\/books\/(?!new\/?$)[^/]+\/?$/.test(pathname)) return 'book'
  return 'other'
}

/** Khung chung: thanh bên ở màn rộng, thanh điều hướng dưới ở điện thoại (ẩn khi đọc/theo dõi truyện). */
export function AppShell({ children }: { children: ReactNode }) {
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const kind = routeKind(pathname)

  if (kind === 'reader') return <div className="min-h-dvh w-full">{children}</div>

  return (
    <div className="mx-auto flex min-h-dvh w-full max-w-7xl">
      <aside
        aria-label={msg.nav.main}
        className="sticky top-0 hidden h-dvh w-60 shrink-0 flex-col gap-1 border-r border-line p-4 lg:flex"
      >
        <span className="mb-4 px-3 font-serif text-xl font-semibold">{msg.appName}</span>
        <Link to="/" activeOptions={{ exact: true }} className={sideItem} activeProps={sideActive}>
          <BookOpen aria-hidden size={20} />
          {msg.nav.library}
        </Link>
        <Link to="/books/new" className={sideItem} activeProps={sideActive}>
          <Plus aria-hidden size={20} />
          {msg.nav.newBook}
        </Link>
        <Link to="/settings/keys" className={sideItem} activeProps={sideActive}>
          <Settings aria-hidden size={20} />
          {msg.nav.settings}
        </Link>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <div className={`mx-auto w-full flex-1 ${kind === 'book' ? 'max-w-6xl' : 'max-w-3xl'}`}>{children}</div>
        {kind === 'other' && (
          <nav
            aria-label={msg.nav.main}
            className="pb-safe sticky bottom-0 flex items-center justify-around gap-2 border-t border-line bg-surface px-4 pt-2.5 lg:hidden"
          >
            <Link to="/" activeOptions={{ exact: true }} className={navItem} activeProps={active}>
              <BookOpen aria-hidden size={22} />
              {msg.nav.library}
            </Link>
            <Link
              to="/books/new"
              className="flex h-12 items-center gap-2 rounded-full bg-accent px-5 text-[15px] font-semibold text-on-accent no-underline hover:bg-accent-hover hover:text-on-accent"
            >
              <Plus aria-hidden size={18} strokeWidth={2.2} />
              {msg.nav.newBook}
            </Link>
            <Link to="/settings/keys" className={navItem} activeProps={active}>
              <Settings aria-hidden size={22} />
              {msg.nav.settings}
            </Link>
          </nav>
        )}
      </div>
    </div>
  )
}

export function PageHeader({ title, back }: { title: string; back?: ReactNode }) {
  return (
    <header className="pt-safe flex items-center gap-1 px-3 pb-1">
      {back}
      <h1 className="m-0 font-serif text-2xl font-semibold">{title}</h1>
    </header>
  )
}
