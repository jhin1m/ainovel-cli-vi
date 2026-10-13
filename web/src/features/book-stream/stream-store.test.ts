import { QueryClient } from '@tanstack/react-query'
import { describe, expect, it, vi } from 'vitest'
import { qk } from '@/api/queries'
import type { Book, BookSnapshot, Event } from '@/api/types.gen'
import { connectBookStream } from './connect'
import { createBookStreamStore, MAX_EVENTS } from './stream-store'

const ev = (id: string, over: Partial<Event> = {}): Event => ({
  id,
  time: '2026-10-13T00:00:00Z',
  category: 'TOOL',
  summary: `s-${id}`,
  depth: 0,
  duration_ms: 0,
  failed: false,
  finished: false,
  ...over,
})

const sync = (fn: () => void) => fn()

describe('stream store', () => {
  it('upserts by id keeping first position', () => {
    const st = createBookStreamStore(sync)
    st.upsertEvent(ev('a'))
    st.upsertEvent(ev('b'))
    st.upsertEvent(ev('a', { finished: true, duration_ms: 1200 }))
    const { events } = st.getSnapshot()
    expect(events.map((e) => e.id)).toEqual(['a', 'b'])
    expect(events[0]?.finished).toBe(true)
  })

  it('caps events and drops the oldest', () => {
    const st = createBookStreamStore(sync)
    for (let i = 0; i < MAX_EVENTS + 5; i++) st.upsertEvent(ev(`e${i}`))
    const { events } = st.getSnapshot()
    expect(events).toHaveLength(MAX_EVENTS)
    expect(events[0]?.id).toBe('e5')
  })

  it('appends stream text and clears on clear', () => {
    const st = createBookStreamStore(sync)
    st.applyStream({ delta: 'Xin ' })
    st.applyStream({ delta: 'chào' })
    expect(st.getSnapshot().streamText).toBe('Xin chào')
    st.applyStream({ clear: true })
    expect(st.getSnapshot().streamText).toBe('')
  })

  it('batches notifications per scheduled flush', () => {
    const queue: (() => void)[] = []
    const st = createBookStreamStore((fn) => queue.push(fn))
    const l = vi.fn()
    st.subscribe(l)
    st.upsertEvent(ev('a'))
    st.upsertEvent(ev('b'))
    expect(l).not.toHaveBeenCalled()
    queue.splice(0).forEach((f) => f())
    expect(l).toHaveBeenCalledTimes(1)
    expect(st.getSnapshot().events).toHaveLength(2)
  })
})

class FakeES {
  static all: FakeES[] = []
  readyState = 1
  closed = false
  onopen: (() => void) | null = null
  onerror: (() => void) | null = null
  private handlers = new Map<string, ((m: MessageEvent) => void)[]>()
  constructor(
    public url: string,
    public init?: EventSourceInit,
  ) {
    FakeES.all.push(this)
  }
  addEventListener(t: string, h: (m: MessageEvent) => void) {
    this.handlers.set(t, [...(this.handlers.get(t) ?? []), h])
  }
  close() {
    this.closed = true
  }
  emit(type: string, data: unknown) {
    this.handlers.get(type)?.forEach((h) => h({ data: JSON.stringify(data) } as MessageEvent))
  }
}

const snap = (completed: number): BookSnapshot => ({ completed_count: completed }) as BookSnapshot
const book = (over: Partial<Book> = {}): Book =>
  ({ id: 'b1', status: 'running', last_error: '', stop_reason: '', snapshot: snap(1), ...over }) as Book

function setup() {
  FakeES.all = []
  const qc = new QueryClient()
  qc.setQueryData(qk.book('b1'), book())
  const store = createBookStreamStore(sync)
  const stop = connectBookStream('b1', store, qc, FakeES as unknown as typeof EventSource)
  const es = FakeES.all[0]!
  return { qc, store, stop, es }
}

describe('connectBookStream', () => {
  it('opens with credentials on the book events url', () => {
    const { es, stop } = setup()
    expect(es.url).toBe('/api/books/b1/events')
    expect(es.init?.withCredentials).toBe(true)
    stop()
    expect(es.closed).toBe(true)
  })

  it('snapshot replaces the cached snapshot and refreshes chapters when count grows', () => {
    const { qc, es } = setup()
    const inv = vi.spyOn(qc, 'invalidateQueries')
    es.emit('snapshot', snap(2))
    expect(qc.getQueryData<Book>(qk.book('b1'))?.snapshot?.completed_count).toBe(2)
    expect(inv).toHaveBeenCalledWith({ queryKey: qk.chapters('b1') })
  })

  it('status updates the book and refreshes chapters when it settles', () => {
    const { qc, es } = setup()
    const inv = vi.spyOn(qc, 'invalidateQueries')
    es.emit('status', { status: 'waiting_review', stop_reason: 'review_wait', last_error: '' })
    expect(qc.getQueryData<Book>(qk.book('b1'))?.status).toBe('waiting_review')
    expect(inv).toHaveBeenCalledWith({ queryKey: qk.chapters('b1') })
  })

  it('upserts events and ignores malformed payloads', () => {
    const { store, es } = setup()
    es.emit('event', ev('x'))
    es.emit('event', ev('x', { finished: true }))
    es.emit('event', { summary: 'no id' })
    es.emit('stream', { delta: 'abc' })
    const s = store.getSnapshot()
    expect(s.events).toHaveLength(1)
    expect(s.events[0]?.finished).toBe(true)
    expect(s.streamText).toBe('abc')
  })

  it('replaces the timeline on every (re)connect and shows reconnecting until reopened', () => {
    const { store, es } = setup()
    es.onopen?.()
    es.emit('event', ev('x'))
    es.emit('stream', { delta: 'abc' })
    es.onerror?.()
    expect(store.getSnapshot().reconnecting).toBe(true)
    expect(store.getSnapshot().events).toHaveLength(1)
    es.onopen?.()
    const s = store.getSnapshot()
    expect(s.reconnecting).toBe(false)
    expect(s.events).toHaveLength(0)
    expect(s.streamText).toBe('')
    es.emit('event', ev('x'))
    expect(store.getSnapshot().events).toHaveLength(1)
  })

  it('reopens manually when the browser gives up', () => {
    vi.useFakeTimers()
    const { es, stop } = setup()
    es.readyState = 2
    es.onerror?.()
    expect(es.closed).toBe(true)
    vi.advanceTimersByTime(3000)
    expect(FakeES.all).toHaveLength(2)
    stop()
    vi.useRealTimers()
  })
})
