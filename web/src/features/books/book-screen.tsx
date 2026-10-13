import { Pause, Play } from 'lucide-react'
import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { errorMessage } from '@/api/client'
import { RESUMABLE } from '@/api/status'
import { useBook, useNextChapter, usePatchBook, usePauseBook, useResumeBook } from '@/api/queries'
import type { Book } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { FormError } from '@/components/ui/field'
import { Dialog } from '@/components/ui/dialog'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { useBookStreamStore, useStreamSelector } from '@/features/book-stream/use-book-stream'
import { msg } from '@/lib/messages'
import { BookActionsMenu } from './book-actions-menu'
import { BookHeader, ExportMenu } from './book-header'
import { BookStatsStrip } from './book-stats-strip'
import { ChaptersTab, OutlineTab } from './book-tabs'
import { BudgetDialog } from './budget-dialog'
import { ChangeKeyPanel } from './change-key-panel'
import { DeleteBookDialog } from './delete-book-dialog'
import { ProgressStepper } from './progress-stepper'
import { StatusBanner } from './status-banner'
import { SteerBox, SteerFab } from './steer-box'
import { useColumnsLayout } from './use-columns-layout'
import { WritingTab } from './writing-tab'

type Tab = 'writing' | 'chapters' | 'outline'

const RUNNING = new Set(['running', 'queued'])

/** Hành động mở sẵn khi vào trang (search param `?action=`): hộp thoại ngân sách hoặc chạy lại. */
export type BookAction = 'budget' | 'retry'

/**
 * Thông báo sau một thao tác; chỉ hiện khi truyện còn ở trạng thái lúc thao tác xong (vừa xếp
 * hàng thì vẫn hiện khi đã chạy, vì queued → running chỉ mất khoảng một giây).
 */
interface Notice {
  text: string
  status: string
  error?: boolean
}

function ReviewSwitch({ book }: { book: Book }) {
  const patch = usePatchBook(book.id)
  const on = book.advance_mode === 'review'
  return (
    <div className="flex items-center justify-between gap-3">
      <div>
        <span id="review-label" className="text-sm font-semibold">{msg.book.reviewMode}</span>
        <p className="m-0 text-xs text-muted">{msg.book.reviewModeHint}</p>
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={on}
        aria-labelledby="review-label"
        disabled={patch.isPending}
        onClick={() => patch.mutate({ advance_mode: on ? 'auto' : 'review' })}
        className="flex min-h-[44px] min-w-[52px] shrink-0 cursor-pointer items-center justify-center border-0 bg-transparent p-0"
      >
        <span
          className={`flex h-8 w-[52px] items-center rounded-2xl p-[3px] ${
            on ? 'justify-end bg-accent' : 'justify-start bg-switch-off'
          }`}
        >
          <span className="size-[26px] rounded-full bg-white shadow-sm" />
        </span>
      </button>
    </div>
  )
}

const tabId = (t: Tab) => `book-tab-${t}`
const panelId = (t: Tab) => `book-panel-${t}`

function Tabs({
  tabs,
  tab,
  onChange,
}: {
  tabs: { id: Tab; label: string }[]
  tab: Tab
  onChange: (t: Tab) => void
}) {
  const refs = useRef(new Map<Tab, HTMLButtonElement>())

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const i = tabs.findIndex((t) => t.id === tab)
    let next: number
    if (e.key === 'ArrowRight') next = (i + 1) % tabs.length
    else if (e.key === 'ArrowLeft') next = (i - 1 + tabs.length) % tabs.length
    else if (e.key === 'Home') next = 0
    else if (e.key === 'End') next = tabs.length - 1
    else return
    e.preventDefault()
    const target = tabs[next]
    if (!target) return
    onChange(target.id)
    refs.current.get(target.id)?.focus()
  }

  return (
    <div role="tablist" aria-label={msg.book.tabs.label} onKeyDown={onKeyDown} className="flex gap-1 rounded-control bg-track p-1">
      {tabs.map((t) => (
        <button
          key={t.id}
          ref={(el) => {
            if (el) refs.current.set(t.id, el)
            else refs.current.delete(t.id)
          }}
          id={tabId(t.id)}
          type="button"
          role="tab"
          aria-selected={tab === t.id}
          aria-controls={panelId(t.id)}
          tabIndex={tab === t.id ? 0 : -1}
          onClick={() => onChange(t.id)}
          className={`min-h-[44px] flex-1 cursor-pointer rounded-md border-0 text-sm ${
            tab === t.id ? 'bg-surface font-semibold text-fg shadow-sm' : 'bg-transparent font-medium text-muted'
          }`}
        >
          {t.label}
        </button>
      ))}
    </div>
  )
}

interface LoadedProps {
  book: Book
  action?: BookAction
  /** Gọi sau khi đã dùng `action` (để xoá nó khỏi URL). */
  onActionUsed?: () => void
}

function Loaded({ book, action, onActionUsed }: LoadedProps) {
  const store = useBookStreamStore(book.id)
  const reconnecting = useStreamSelector(store, (st) => st.reconnecting)
  const failure = useStreamSelector(store, (st) => st.failure)
  const pause = usePauseBook(book.id)
  const resume = useResumeBook(book.id)
  const next = useNextChapter(book.id)
  const [tab, setTab] = useState<Tab>('writing')
  const [budgetOpen, setBudgetOpen] = useState(() => action === 'budget' && !book.subscription)
  const [keyOpen, setKeyOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [notice, setNotice] = useState<Notice | null>(null)
  const showNotice = (text: string, status: string) => setNotice({ text, status })
  const showNoticeError = (text: string, status: string) => setNotice({ text, status, error: true })
  const busy = pause.isPending || resume.isPending || next.isPending
  const cmdError = pause.error ?? resume.error ?? next.error
  const done = book.snapshot?.completed_count ?? 0
  const writing = tab === 'writing'
  const chromeRef = useRef<HTMLDivElement>(null)
  const columns = useColumnsLayout(chromeRef, writing)

  // `action` chỉ dùng một lần khi mở trang: "budget" đã mở hộp thoại qua state ban đầu ở trên,
  // "retry" chạy lại nếu server cho phép; xong thì xoá khỏi URL để tải lại trang không lặp lại.
  const actionUsed = useRef(false)
  const resumeNow = resume.mutate
  const statusAtOpen = useRef(book.status)
  useEffect(() => {
    if (!action || actionUsed.current) return
    actionUsed.current = true
    if (action === 'retry' && RESUMABLE.has(statusAtOpen.current)) resumeNow()
    onActionUsed?.()
  }, [action, onActionUsed, resumeNow])

  const tabs: { id: Tab; label: string }[] = [
    { id: 'writing', label: msg.book.tabs.writing },
    { id: 'chapters', label: done > 0 ? `${msg.book.tabs.chapters} (${done})` : msg.book.tabs.chapters },
    { id: 'outline', label: msg.book.tabs.outline },
  ]

  return (
    // Bố cục 2 cột (tab "Đang viết", màn ≥1280px đủ cao): khung cao đúng một màn hình, hai cột
    // tự cuộn, trang không cuộn. Không đủ chỗ thì trang xếp dọc và cuộn bình thường.
    <div className={`flex min-h-dvh flex-col ${columns ? 'h-dvh' : ''}`}>
      {failure ? (
        <div role="alert" className="bg-st-error-bg px-4 py-2 text-center text-sm font-medium text-st-error">
          {msg.book.streamStopped}: {failure}
        </div>
      ) : (
        reconnecting && (
          <div role="status" className="bg-st-budget-bg px-4 py-2 text-center text-sm font-medium text-st-budget">
            {msg.book.reconnecting}
          </div>
        )
      )}
      {/* Mobile: chừa chỗ dưới cho nút "Góp ý" nổi. Từ md lề ngang do AppShell lo. */}
      <div className={`flex flex-1 flex-col gap-4 px-4 pb-24 md:px-0 md:pb-6 ${columns ? 'min-h-0' : ''}`}>
        <div ref={chromeRef} className="flex flex-col gap-4">
          <BookHeader
            book={book}
            actions={
              <BookActionsMenu
                disabled={book.status === 'deleting'}
                items={[
                  { id: 'key', label: msg.book.menu.changeKey, onSelect: () => setKeyOpen(true) },
                  { id: 'delete', label: msg.book.menu.delete, onSelect: () => setDeleteOpen(true), danger: true },
                ]}
              />
            }
            controls={
              <>
                {RUNNING.has(book.status) && (
                  <Button variant="secondary" disabled={busy} onClick={() => pause.mutate()}>
                    <Pause aria-hidden size={16} />
                    {msg.book.pause}
                  </Button>
                )}
                {/* Provider đã từ chối hẳn (sai gói): tiếp tục chỉ lặp lại lỗi, banner có lối ra. */}
                {RESUMABLE.has(book.status) && book.stop_reason !== 'provider_rejected' && (
                  <Button disabled={busy} onClick={() => resume.mutate()}>
                    <Play aria-hidden size={16} />
                    {msg.book.resume}
                  </Button>
                )}
                <ExportMenu bookId={book.id} disabled={done === 0} />
              </>
            }
          />

          <BookStatsStrip book={book} onBudget={book.subscription ? undefined : () => setBudgetOpen(true)} />

          {cmdError && <FormError>{errorMessage(cmdError)}</FormError>}
          {notice &&
            (notice.status === book.status || (notice.status === 'queued' && book.status === 'running')) &&
            (notice.error ? (
              <FormError>{notice.text}</FormError>
            ) : (
              <p role="status" className="m-0 text-sm font-medium">
                {notice.text}
              </p>
            ))}

          <StatusBanner
            book={book}
            busy={busy}
            onNext={() => next.mutate()}
            onResume={() => resume.mutate()}
            onRaiseBudget={() => setBudgetOpen(true)}
            onNotice={showNotice}
            onNoticeError={showNoticeError}
          />

          <ProgressStepper book={book} aside={<ReviewSwitch book={book} />} />

          <Tabs tabs={tabs} tab={tab} onChange={setTab} />
        </div>

        {writing && (
          <div
            role="tabpanel"
            id={panelId('writing')}
            aria-labelledby={tabId('writing')}
            className={columns ? 'min-h-0 flex-1' : undefined}
          >
            <WritingTab book={book} store={store} columns={columns} />
          </div>
        )}
        {tab === 'chapters' && (
          <div role="tabpanel" id={panelId('chapters')} aria-labelledby={tabId('chapters')}>
            <ChaptersTab bookId={book.id} />
          </div>
        )}
        {tab === 'outline' && (
          <div role="tabpanel" id={panelId('outline')} aria-labelledby={tabId('outline')}>
            <OutlineTab bookId={book.id} active />
          </div>
        )}
        {/* Tab "Đang viết" tự đặt ô góp ý cạnh bản nháp; các tab khác đặt cuối trang (từ 768px). */}
        {!writing && (
          <div className="hidden md:block">
            <SteerBox bookId={book.id} pending={book.pending_text} disabled={book.status === 'deleting'} />
          </div>
        )}
      </div>

      <SteerFab book={book} />

      <BudgetDialog book={book} open={budgetOpen} onClose={() => setBudgetOpen(false)} onSaved={showNotice} />
      <Dialog open={keyOpen} title={msg.book.changeKey.title} onClose={() => setKeyOpen(false)}>
        <ChangeKeyPanel
          book={book}
          onDone={(text, status) => {
            setKeyOpen(false)
            showNotice(text, status)
          }}
        />
        {/* Điện thoại không có Esc: luôn có nút đóng, kể cả khi chưa có key nào để chọn. */}
        <div className="mt-4 flex justify-end">
          <Button variant="ghost" onClick={() => setKeyOpen(false)}>
            {msg.book.changeKey.close}
          </Button>
        </div>
      </Dialog>
      <DeleteBookDialog book={book} open={deleteOpen} onClose={() => setDeleteOpen(false)} />
    </div>
  )
}

interface BookScreenProps {
  id: string
  action?: BookAction
  onActionUsed?: () => void
}

export function BookScreen({ id, action, onActionUsed }: BookScreenProps) {
  const book = useBook(id)
  if (book.data) return <Loaded book={book.data} action={action} onActionUsed={onActionUsed} />
  if (book.isPending) return <LoadingState />
  return (
    <div className="p-5">
      <ErrorState title={msg.book.loadError} detail={errorMessage(book.error)} onRetry={() => void book.refetch()} />
    </div>
  )
}
