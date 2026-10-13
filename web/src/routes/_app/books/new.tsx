import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { PageHeader } from '@/components/app-shell'
import { NewBookForm } from '@/features/books/new-book-form'
import { msg } from '@/lib/messages'

export const Route = createFileRoute('/_app/books/new')({ component: NewBookPage })

function NewBookPage() {
  const navigate = useNavigate()
  return (
    <>
      <PageHeader
        title={msg.newBook.title}
        back={
          <Link to="/" aria-label={msg.back} className="flex size-11 items-center justify-center text-fg">
            <ArrowLeft aria-hidden size={22} />
          </Link>
        }
      />
      <NewBookForm onCreated={(b) => void navigate({ to: '/books/$id', params: { id: b.id } })} />
    </>
  )
}
