import { createFileRoute } from '@tanstack/react-router'
import { BookScreen } from '@/features/books/book-screen'

export const Route = createFileRoute('/_app/books/$id/')({ component: BookPage })

function BookPage() {
  const { id } = Route.useParams()
  return <BookScreen key={id} id={id} />
}
