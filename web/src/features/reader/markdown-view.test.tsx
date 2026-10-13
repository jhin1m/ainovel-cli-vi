import { render } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { MarkdownView } from './markdown-view'
import { DEFAULT_SETTINGS, loadSettings, saveSettings, STORAGE_KEY } from './reader-settings'

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

describe('reader settings', () => {
  afterEach(() => localStorage.clear())

  it('round-trips and clamps stored values', () => {
    saveSettings({ size: 20, font: 'sans', theme: 'dark' })
    expect(loadSettings()).toEqual({ size: 20, font: 'sans', theme: 'dark' })
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ size: 999, font: 'x', theme: 'neon' }))
    expect(loadSettings()).toEqual({ ...DEFAULT_SETTINGS, size: 24 })
  })

  it('falls back to defaults on corrupt storage', () => {
    localStorage.setItem(STORAGE_KEY, '{oops')
    expect(loadSettings()).toEqual(DEFAULT_SETTINGS)
  })
})
