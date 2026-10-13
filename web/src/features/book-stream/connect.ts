import type { QueryClient } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import { bookQuery, qk } from '@/api/queries'
import type { Book, BookSnapshot, Event, StatusChange, StreamDelta } from '@/api/types.gen'
import { msg } from '@/lib/messages'
import type { BookStreamStore } from './stream-store'

type EventSourceCtor = new (url: string, init?: EventSourceInit) => EventSource

const RETRY_MIN_MS = 3000
const RETRY_MAX_MS = 30_000
const SETTLED = new Set(['waiting_review', 'completed', 'budget_exhausted'])

function parse(raw: unknown): unknown {
  if (typeof raw !== 'string') return undefined
  try {
    return JSON.parse(raw)
  } catch {
    return undefined
  }
}

const isObject = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null

/**
 * Nối EventSource của một truyện vào store và cache TanStack Query.
 * Trả về hàm huỷ. Khi trình duyệt bỏ cuộc (CLOSED) thì tự mở lại sau một khoảng chờ tăng dần.
 */
export function connectBookStream(
  id: string,
  store: BookStreamStore,
  qc: QueryClient,
  ES: EventSourceCtor = EventSource,
): () => void {
  const url = `/api/books/${encodeURIComponent(id)}/events`
  const bookKey = qk.book(id)
  let es: EventSource | null = null
  let timer: ReturnType<typeof setTimeout> | undefined
  let delay = RETRY_MIN_MS
  let stopped = false

  const patchBook = (fn: (b: Book) => Book) => {
    qc.setQueryData<Book>(bookKey, (old) => (old ? fn(old) : old))
  }

  const onSnapshot = (data: unknown) => {
    if (!isObject(data)) return
    const snap = data as unknown as BookSnapshot
    const prev = qc.getQueryData<Book>(bookKey)?.snapshot
    patchBook((b) => ({ ...b, snapshot: snap }))
    if (prev && snap.completed_count > prev.completed_count) {
      void qc.invalidateQueries({ queryKey: qk.chapters(id) })
    }
  }

  const onStatus = (data: unknown) => {
    if (!isObject(data) || typeof data.status !== 'string') return
    const ch = data as unknown as StatusChange
    const prev = qc.getQueryData<Book>(bookKey)
    patchBook((b) => ({
      ...b,
      status: ch.status,
      stop_reason: typeof ch.stop_reason === 'string' ? ch.stop_reason : b.stop_reason,
      last_error: typeof ch.last_error === 'string' ? ch.last_error : b.last_error,
    }))
    if (prev && prev.status !== ch.status) {
      void qc.invalidateQueries({ queryKey: qk.books })
      void qc.invalidateQueries({ queryKey: bookKey, exact: true })
      if (SETTLED.has(ch.status)) void qc.invalidateQueries({ queryKey: qk.chapters(id) })
    }
  }

  const scheduleRetry = () => {
    timer = setTimeout(open, delay)
    delay = Math.min(delay * 2, RETRY_MAX_MS)
  }

  // Trình duyệt chỉ báo CLOSED khi server từ chối (401/404/...) nên hỏi lại truyện để biết lý do thật.
  const recover = async () => {
    try {
      await qc.fetchQuery({ ...bookQuery(id), staleTime: 0 })
    } catch (e) {
      if (stopped) return
      const status = e instanceof ApiError ? e.status : 0
      const transient = status === 0 || status === 429 || status >= 500
      if (!transient) {
        // 401 đã được client API đưa về luồng đăng nhập; 404 nghĩa là truyện không còn.
        if (status === 404) void qc.invalidateQueries({ queryKey: qk.books })
        store.setFailure(e instanceof ApiError ? e.message : msg.genericError)
        return
      }
    }
    if (!stopped) scheduleRetry()
  }

  const open = () => {
    if (stopped) return
    const source = new ES(url, { withCredentials: true })
    es = source
    source.onopen = () => {
      delay = RETRY_MIN_MS
      store.reset()
    }
    source.onerror = () => {
      store.setReconnecting(true)
      if (source.readyState === 2 /* CLOSED */ && !stopped) {
        source.close()
        void recover()
      }
    }
    source.addEventListener('snapshot', (m) => onSnapshot(parse((m as MessageEvent).data)))
    source.addEventListener('status', (m) => onStatus(parse((m as MessageEvent).data)))
    source.addEventListener('event', (m) => {
      const e = parse((m as MessageEvent).data)
      if (isObject(e) && typeof e.id === 'string' && e.id !== '') store.upsertEvent(e as unknown as Event)
    })
    source.addEventListener('stream', (m) => {
      const d = parse((m as MessageEvent).data)
      if (isObject(d)) store.applyStream(d as StreamDelta)
    })
  }

  open()
  return () => {
    stopped = true
    clearTimeout(timer)
    es?.close()
  }
}
