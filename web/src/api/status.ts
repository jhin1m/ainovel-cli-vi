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

export function isKnownStatus(s: BookStatus): s is KnownBookStatus {
  return (BOOK_STATUSES as readonly string[]).includes(s)
}
