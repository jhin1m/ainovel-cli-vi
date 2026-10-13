import { memo, useEffect, useMemo, useRef, useState } from 'react'
import { msg } from '@/lib/messages'

const Paragraph = memo(function Paragraph({ text }: { text: string }) {
  return <p className="draft-para m-0 mb-3 whitespace-pre-wrap">{text}</p>
})

interface Props {
  text: string
  /** Lớp cho khung ngoài; khung tự cuộn được thì bản nháp tự bám đáy. */
  className?: string
  /**
   * Chỉ hiện `tail` đoạn cuối (kèm nút xem toàn bộ) khi khung không tự cuộn, để chữ mới luôn
   * nằm gần đầu khung và phần bên dưới không bị đẩy xa theo cả chương.
   */
  tail?: number
}

/**
 * Bản nháp đang stream, chỉ là text node. Đoạn đã xong được memo nên không render lại.
 * Không tự đặt chiều cao: `className` quyết định khung có tự cuộn hay không.
 */
export function StreamText({ text, className = '', tail }: Props) {
  const paragraphs = useMemo(() => text.split(/\n{2,}/).filter((p) => p.trim() !== ''), [text])
  const boxRef = useRef<HTMLDivElement>(null)
  const pinned = useRef(true)
  const [expanded, setExpanded] = useState(false)
  const hidden = tail !== undefined && !expanded ? Math.max(0, paragraphs.length - tail) : 0

  useEffect(() => {
    const el = boxRef.current
    if (el && pinned.current) el.scrollTop = el.scrollHeight
  }, [text])

  if (paragraphs.length === 0) {
    return <p className="m-0 text-sm text-muted">{msg.book.writing.draftEmpty}</p>
  }
  return (
    <div
      ref={boxRef}
      onScroll={(e) => {
        const el = e.currentTarget
        pinned.current = el.scrollHeight - el.scrollTop - el.clientHeight < 48
      }}
      className={`pr-1 font-serif text-lg leading-[1.7] ${className}`}
    >
      <div className="max-w-[72ch]">
        {tail !== undefined && (hidden > 0 || expanded) && (
          <button
            type="button"
            aria-expanded={expanded}
            onClick={() => setExpanded((v) => !v)}
            className="mb-3 min-h-[44px] cursor-pointer rounded-control border-0 bg-transparent px-0 font-sans text-sm font-semibold text-accent"
          >
            {expanded ? msg.book.writing.collapse : msg.book.writing.showAll(hidden)}
          </button>
        )}
        {paragraphs.slice(hidden).map((p, i) => (
          <Paragraph key={hidden + i} text={p} />
        ))}
      </div>
    </div>
  )
}
