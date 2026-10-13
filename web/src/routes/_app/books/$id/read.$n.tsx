import { createFileRoute } from '@tanstack/react-router'
import { ReaderPage } from '@/features/reader/reader-page'

export const Route = createFileRoute('/_app/books/$id/read/$n')({ component: ReadPage })

function ReadPage() {
  const { id, n } = Route.useParams()
  return <ReaderPage key={id} bookId={id} n={/^[1-9]\d{0,8}$/.test(n) ? Number(n) : Number.NaN} />
}
