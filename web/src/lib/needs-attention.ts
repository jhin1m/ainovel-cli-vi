import type { KnownBookStatus } from '@/api/status'
import type { Book } from '@/api/types.gen'

/** Trạng thái cần user làm gì đó: duyệt, sửa lỗi, nâng ngân sách, chạy lại. */
export const ATTENTION_STATUSES: readonly KnownBookStatus[] = [
  'waiting_review',
  'failed',
  'budget_exhausted',
  'interrupted',
]

/** Các truyện đang chờ user xử lý. Dùng cho tiền tố "(n)" trên tab và dải "Cần bạn xử lý". */
export function needsAttention(books: readonly Book[]): Book[] {
  return books.filter((b) => (ATTENTION_STATUSES as readonly string[]).includes(b.status))
}
