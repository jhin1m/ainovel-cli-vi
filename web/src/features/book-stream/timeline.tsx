import { useVirtualizer } from '@tanstack/react-virtual'
import { ArrowDown } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import type { Event } from '@/api/types.gen'
import { Spinner } from '@/components/ui/states'
import { msg } from '@/lib/messages'
import { agentMeta } from './agent-meta'

function meta(e: Event): string {
  if (!e.finished || e.duration_ms <= 0) return ''
  return msg.book.writing.seconds((e.duration_ms / 1000).toFixed(1).replace('.', ','))
}

export function EventRow({ event }: { event: Event }) {
  const a = agentMeta(event.agent)
  const m = meta(event)
  return (
    <div className="flex gap-3 border-b border-line px-1 py-2.5" style={{ paddingLeft: `${Math.min(event.depth, 3) * 12 + 4}px` }}>
      <span className={`mt-0.5 h-6 shrink-0 rounded-full px-2.5 text-xs leading-6 font-semibold ${a.tone}`}>{a.label}</span>
      <div className="min-w-0 flex-1">
        <p className={`m-0 text-[14px] leading-snug break-words ${event.failed ? 'text-st-error' : ''}`}>
          {event.summary}
          {event.failed && <span className="sr-only"> ({msg.book.writing.failedStep})</span>}
        </p>
        {m && <span className="text-[12px] text-muted">{m}</span>}
        {event.detail && (
          <details className="mt-1">
            <summary className="cursor-pointer text-[12px] text-muted">{msg.book.writing.detail}</summary>
            <pre className="m-0 mt-1 max-h-48 overflow-auto text-[12px] break-words whitespace-pre-wrap text-muted">{event.detail}</pre>
          </details>
        )}
      </div>
      {!event.finished && <Spinner className="mt-1 shrink-0 text-muted" />}
    </div>
  )
}

/** Danh sách hoạt động ảo hoá, tự cuộn xuống đáy khi đang ở đáy. */
export function Timeline({ events }: { events: readonly Event[] }) {
  const parentRef = useRef<HTMLDivElement>(null)
  const [atBottom, setAtBottom] = useState(true)
  const stick = useRef(true)

  // useVirtualizer trả hàm không memo được; component này không bọc memo nên an toàn.
  // eslint-disable-next-line react-hooks/incompatible-library
  const virt = useVirtualizer({
    count: events.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 56,
    overscan: 8,
    getItemKey: (i) => events[i]?.id ?? i,
  })

  const jump = useCallback(() => {
    if (events.length > 0) virt.scrollToIndex(events.length - 1, { align: 'end' })
    stick.current = true
    setAtBottom(true)
  }, [events.length, virt])

  useEffect(() => {
    if (stick.current && events.length > 0) virt.scrollToIndex(events.length - 1, { align: 'end' })
  }, [events, virt])

  if (events.length === 0) {
    return <p className="m-0 py-6 text-center text-sm text-muted">{msg.book.writing.activityEmpty}</p>
  }

  return (
    <div className="relative">
      <div
        ref={parentRef}
        role="log"
        aria-label={msg.book.writing.activity}
        onScroll={(e) => {
          const el = e.currentTarget
          const bottom = el.scrollHeight - el.scrollTop - el.clientHeight < 48
          stick.current = bottom
          setAtBottom(bottom)
        }}
        className="h-[48dvh] min-h-[280px] overflow-y-auto lg:h-[56dvh]"
      >
        <div className="relative w-full" style={{ height: virt.getTotalSize() }}>
          {virt.getVirtualItems().map((v) => {
            const ev = events[v.index]
            if (!ev) return null
            return (
            <div
              key={v.key}
              data-index={v.index}
              ref={virt.measureElement}
              className="absolute top-0 left-0 w-full"
              style={{ transform: `translateY(${v.start}px)` }}
            >
              <EventRow event={ev} />
            </div>
            )
          })}
        </div>
      </div>
      {!atBottom && (
        <button
          type="button"
          onClick={jump}
          className="absolute right-3 bottom-3 inline-flex min-h-[44px] cursor-pointer items-center gap-1.5 rounded-full border-0 bg-accent px-4 text-sm font-semibold text-on-accent shadow-md"
        >
          <ArrowDown aria-hidden size={16} />
          {msg.book.writing.latest}
        </button>
      )}
    </div>
  )
}
