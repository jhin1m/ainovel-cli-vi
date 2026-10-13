import { createFileRoute } from '@tanstack/react-router'
import { BookScreen, type BookAction } from '@/features/books/book-screen'

interface BookSearch {
  /** Hành động tự làm khi mở trang (từ dải "Cần bạn xử lý"); xoá khỏi URL sau khi dùng. */
  action?: BookAction
}

export const Route = createFileRoute('/_app/books/$id/')({
  validateSearch: (s: Record<string, unknown>): BookSearch =>
    s.action === 'budget' || s.action === 'retry' ? { action: s.action } : {},
  component: BookPage,
})

function BookPage() {
  const { id } = Route.useParams()
  const { action } = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <BookScreen
      key={id}
      id={id}
      action={action}
      onActionUsed={() => void navigate({ search: {}, replace: true })}
    />
  )
}
