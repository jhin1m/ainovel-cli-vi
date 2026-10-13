import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { MarkdownView } from './markdown-view'

describe('MarkdownView', () => {
  it('does not turn raw HTML into elements', () => {
    const { container } = render(
      <MarkdownView content={'Mở đầu\n\n<img src=x onerror="window.__pwned=1">\n\n<script>window.__pwned=1</script>\n\nKết'} />,
    )
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('[onerror]')).toBeNull()
    expect((window as unknown as { __pwned?: number }).__pwned).toBeUndefined()
    expect(container.textContent).toContain('Mở đầu')
  })

  it('drops markdown images and unsafe links', () => {
    const { container } = render(
      <MarkdownView content={'![ảnh](https://x.test/a.png) [bad](javascript:alert(1)) [ok](https://example.com)'} />,
    )
    expect(container.querySelector('img')).toBeNull()
    const links = container.querySelectorAll('a')
    expect(links).toHaveLength(1)
    expect(links[0]?.getAttribute('href')).toBe('https://example.com')
    expect(links[0]?.getAttribute('rel')).toBe('noopener noreferrer')
  })
})

