import { memo, useEffect, useMemo, useRef } from 'react'
import { msg } from '@/lib/messages'

const Paragraph = memo(function Paragraph({ text }: { text: string }) {
  return <p className="draft-para m-0 mb-3 whitespace-pre-wrap">{text}</p>
})

/** Bản nháp đang stream, chỉ là text node. Đoạn đã xong được memo nên không render lại. */
export function StreamText({ text }: { text: string }) {
  const paragraphs = useMemo(() => text.split(/\n{2,}/).filter((p) => p.trim() !== ''), [text])
  const boxRef = useRef<HTMLDivElement>(null)
  const pinned = useRef(true)

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
      className="max-h-[42dvh] overflow-y-auto pr-1 font-serif text-[17px] leading-[1.7]"
    >
      {paragraphs.map((p, i) => (
        <Paragraph key={i} text={p} />
      ))}
    </div>
  )
}
