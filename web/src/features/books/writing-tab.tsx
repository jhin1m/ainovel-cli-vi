import type { Book } from '@/api/types.gen'
import type { BookStreamStore } from '@/features/book-stream/stream-store'
import { StreamText } from '@/features/book-stream/stream-text'
import { Timeline } from '@/features/book-stream/timeline'
import { selectEvents, selectStreamText, useStreamSelector } from '@/features/book-stream/use-book-stream'
import { msg } from '@/lib/messages'
import { SteerBox } from './steer-box'

const card = 'rounded-card border border-line-card bg-surface shadow-card'
/** Số đoạn cuối của bản nháp được hiện khi trang xếp dọc. */
const DRAFT_TAIL = 4

function DraftSection({ store, columns }: { store: BookStreamStore; columns: boolean }) {
  const text = useStreamSelector(store, selectStreamText)
  return (
    <section className={`${card} flex flex-col p-4 ${columns ? 'min-h-0 flex-1' : ''}`}>
      <h2 className="m-0 mb-2 text-sm font-semibold text-muted">{msg.book.writing.draft}</h2>
      {columns ? (
        <StreamText text={text} className="min-h-0 flex-1 overflow-y-auto" />
      ) : (
        <StreamText text={text} tail={DRAFT_TAIL} />
      )}
    </section>
  )
}

function ActivitySection({ store, columns }: { store: BookStreamStore; columns: boolean }) {
  const events = useStreamSelector(store, selectEvents)
  return (
    <section className={`${card} flex flex-col p-3 ${columns ? 'min-h-0' : ''}`}>
      <h2 className="m-0 mb-1 px-1 text-sm font-semibold text-muted">{msg.book.writing.activity}</h2>
      {columns ? <Timeline events={events} className="min-h-0 flex-1" /> : <Timeline events={events} />}
    </section>
  )
}

/**
 * Tab "Đang viết".
 * - `columns` (màn ≥1280px đủ cao, xem `useColumnsLayout`): 2 cột bản nháp | hoạt động, mỗi cột tự
 *   cuộn trong phần cao còn lại của màn; trang không cuộn nên không có vùng cuộn lồng.
 * - Còn lại: bản nháp (chỉ vài đoạn cuối) rồi hoạt động, cuộn trang. Hoạt động vẫn là khung cuộn
 *   riêng vì danh sách được ảo hoá.
 * Ô góp ý inline chỉ từ 768px; điện thoại dùng nút "Góp ý" (`SteerFab`) ở cấp trang.
 */
export function WritingTab({ book, store, columns }: { book: Book; store: BookStreamStore; columns: boolean }) {
  return (
    <div
      className={`grid gap-4 ${
        columns ? 'h-full min-h-0 grid-cols-[minmax(0,3fr)_minmax(0,2fr)] grid-rows-[minmax(0,1fr)]' : 'grid-cols-1'
      }`}
    >
      <div className={`flex min-w-0 flex-col gap-4 ${columns ? 'min-h-0' : ''}`}>
        <DraftSection store={store} columns={columns} />
        <div className="hidden md:block">
          <SteerBox bookId={book.id} pending={book.pending_text} disabled={book.status === 'deleting'} />
        </div>
      </div>
      <ActivitySection store={store} columns={columns} />
    </div>
  )
}
