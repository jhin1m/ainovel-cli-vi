import type { Event } from '@/api/types.gen'

export const MAX_EVENTS = 2000
const MAX_STREAM_CHARS = 200_000

export interface BookStreamState {
  events: readonly Event[]
  streamText: string
  reconnecting: boolean
}

type Schedule = (fn: () => void) => void

const frameSchedule: Schedule = (fn) => {
  if (typeof requestAnimationFrame === 'function') requestAnimationFrame(() => fn())
  else setTimeout(fn, 16)
}

/**
 * Store ngoài React cho timeline + văn bản stream của một truyện. Thay đổi được gom
 * và báo cho subscriber tối đa một lần mỗi frame; snapshot luôn bất biến.
 */
export function createBookStreamStore(schedule: Schedule = frameSchedule) {
  const events = new Map<string, Event>()
  let streamText = ''
  let reconnecting = false
  let state: BookStreamState = { events: [], streamText: '', reconnecting: false }
  let dirty = false
  let queued = false
  const listeners = new Set<() => void>()

  const publish = () => {
    queued = false
    if (!dirty) return
    dirty = false
    state = { events: [...events.values()], streamText, reconnecting }
    listeners.forEach((l) => l())
  }

  const touch = () => {
    dirty = true
    if (queued) return
    queued = true
    schedule(publish)
  }

  return {
    subscribe(l: () => void) {
      listeners.add(l)
      return () => {
        listeners.delete(l)
      }
    },
    getSnapshot: (): BookStreamState => state,

    /** Mỗi lần kết nối (kể cả kết nối lại) server gửi lại toàn bộ: bỏ dữ liệu cũ. */
    reset() {
      events.clear()
      streamText = ''
      reconnecting = false
      touch()
    },
    setReconnecting(v: boolean) {
      if (reconnecting === v) return
      reconnecting = v
      touch()
    },
    /** Upsert theo id, giữ vị trí lần đầu xuất hiện; vượt giới hạn thì bỏ event cũ nhất. */
    upsertEvent(e: Event) {
      events.set(e.id, e)
      if (events.size > MAX_EVENTS) {
        const oldest = events.keys().next()
        if (!oldest.done) events.delete(oldest.value)
      }
      touch()
    },
    applyStream(delta: { delta?: string; clear?: boolean }) {
      if (delta.clear) streamText = ''
      if (delta.delta) {
        streamText += delta.delta
        if (streamText.length > MAX_STREAM_CHARS) streamText = streamText.slice(-MAX_STREAM_CHARS)
      }
      touch()
    },
  }
}

export type BookStreamStore = ReturnType<typeof createBookStreamStore>
