import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { errorMessage } from '@/api/client'
import { forgetBook, useDeleteBook } from '@/api/queries'
import type { Book } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { FormError } from '@/components/ui/field'
import { msg } from '@/lib/messages'
import { bookTitle } from './book-card'

/**
 * Xác nhận xoá truyện. Không dùng <form> nên Enter không kích hoạt nút xoá; focus đầu tiên
 * nằm ở khung hộp thoại, không ở nút nguy hiểm. Xoá xong về thư viện.
 */
function Body({ book, onClose }: { book: Book; onClose: () => void }) {
  const del = useDeleteBook(book.id)
  const qc = useQueryClient()
  const navigate = useNavigate()

  // Dùng mutateAsync: đóng hộp thoại (Esc) lúc đang xoá thì callback của mutate() không chạy,
  // còn promise vẫn chạy tiếp nên vẫn rời trang của truyện đã xoá.
  const confirm = async () => {
    try {
      await del.mutateAsync()
    } catch {
      return // lỗi hiện qua del.error
    }
    await navigate({ to: '/' })
    forgetBook(qc, book.id)
  }

  return (
    <div className="flex flex-col gap-4">
      <p className="m-0 text-base leading-relaxed break-words">{msg.book.del.body(bookTitle(book))}</p>
      {del.isError && <FormError>{errorMessage(del.error)}</FormError>}
      <div className="flex flex-wrap justify-end gap-2">
        <Button variant="ghost" disabled={del.isPending || del.isSuccess} onClick={onClose}>
          {msg.cancel}
        </Button>
        <Button variant="danger" disabled={del.isPending || del.isSuccess} onClick={() => void confirm()}>
          {del.isPending ? msg.book.del.submitting : msg.book.del.submit}
        </Button>
      </div>
    </div>
  )
}

/** Nội dung được mount lại mỗi lần mở nên không giữ lỗi của lần trước. */
export function DeleteBookDialog({ book, open, onClose }: { book: Book; open: boolean; onClose: () => void }) {
  return (
    <Dialog open={open} title={msg.book.del.title} onClose={onClose}>
      <Body book={book} onClose={onClose} />
    </Dialog>
  )
}
