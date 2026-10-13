import type { ComponentProps } from 'react'
import ReactMarkdown from 'react-markdown'
import { msg } from '@/lib/messages'

function isSafeHref(href: string | undefined): href is string {
  if (!href) return false
  try {
    const u = new URL(href)
    return u.protocol === 'http:' || u.protocol === 'https:'
  } catch {
    return false
  }
}

function SafeLink({ href, children }: ComponentProps<'a'>) {
  if (!isSafeHref(href)) return <span>{children}</span>
  return (
    <a href={href} target="_blank" rel="noopener noreferrer">
      {children}
    </a>
  )
}

/**
 * Markdown do LLM sinh ra là dữ liệu không tin cậy: không bật rehype-raw nên HTML
 * thô chỉ hiện như văn bản; ảnh bị bỏ; link chỉ nhận http/https.
 */
export function MarkdownView({ content }: { content: string }) {
  return (
    <ReactMarkdown
      components={{
        a: SafeLink,
        img: ({ alt }) => <span>{alt || msg.reader.imageOmitted}</span>,
      }}
    >
      {content}
    </ReactMarkdown>
  )
}
