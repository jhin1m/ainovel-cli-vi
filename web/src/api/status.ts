import type { BookStatus } from './types.gen'

/** Các giá trị books.status mà server có thể trả. tygo chỉ sinh `string`. */
export const BOOK_STATUSES = [
  'queued',
  'running',
  'waiting_review',
  'paused',
  'budget_exhausted',
  'completed',
  'failed',
  'interrupted',
  'deleting',
] as const

export type KnownBookStatus = (typeof BOOK_STATUSES)[number]

/** Trạng thái server cho phép resume (books/handlers.go handleResume). */
export const RESUMABLE: ReadonlySet<string> = new Set<KnownBookStatus>(['paused', 'failed', 'interrupted'])

export function isKnownStatus(s: BookStatus): s is KnownBookStatus {
  return (BOOK_STATUSES as readonly string[]).includes(s)
}
