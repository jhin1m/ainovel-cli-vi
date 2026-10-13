import { BookOpen } from 'lucide-react'
import type { ReactNode } from 'react'
import { msg } from '@/lib/messages'

export function AuthLayout({
  title,
  subtitle,
  children,
}: {
  title: string
  subtitle: string
  children: ReactNode
}) {
  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-md flex-col justify-center gap-6 px-5 py-8">
      <div className="flex items-center gap-2.5">
        <BookOpen aria-hidden className="text-accent" size={28} strokeWidth={1.8} />
        <span className="text-[19px] font-bold tracking-tight">{msg.appName}</span>
      </div>
      <div>
        <h1 className="m-0 font-serif text-[32px] font-semibold tracking-tight">{title}</h1>
        <p className="mt-1.5 mb-0 text-sm text-muted">{subtitle}</p>
      </div>
      {children}
    </main>
  )
}
