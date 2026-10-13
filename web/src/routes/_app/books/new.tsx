import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { bookQuery } from '@/api/queries'
import { PageHeader } from '@/components/app-shell'
import { LoadingState } from '@/components/ui/states'
import { NewBookForm } from '@/features/books/new-book-form'
import { msg } from '@/lib/messages'

interface NewBookSearch {
  /** Truyện cũ để điền sẵn ý tưởng (vd. tạo lại với provider khác). */
  from?: string
}

export const Route = createFileRoute('/_app/books/new')({
  validateSearch: (s: Record<string, unknown>): NewBookSearch =>
    typeof s.from === 'string' && s.from !== '' ? { from: s.from } : {},
  component: NewBookPage,
})

function NewBookPage() {
  const navigate = useNavigate()
  const { from } = Route.useSearch()
  // Lấy prompt của truyện cũ nếu được; lỗi (vd. truyện đã bị xoá) thì để form trống.
  const source = useQuery({ ...bookQuery(from ?? ''), enabled: Boolean(from), retry: false })
  return (
    <div className="mx-auto w-full max-w-6xl">
      <PageHeader
        title={msg.newBook.title}
        back={
          <Link to="/" aria-label={msg.back} className="flex size-11 items-center justify-center text-fg">
            <ArrowLeft aria-hidden size={22} />
          </Link>
        }
      />
      {from && source.isPending ? (
        <LoadingState />
      ) : (
        <NewBookForm
          initialIdea={source.data?.prompt}
          avoidProvider={source.data?.provider}
          onCreated={(b) => void navigate({ to: '/books/$id', params: { id: b.id } })}
        />
      )}
    </div>
  )
}
