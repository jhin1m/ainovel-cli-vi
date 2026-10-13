import { screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { APIKey, Book, BookSnapshot } from '@/api/types.gen'
import { renderWithProviders } from '@/test/render'
import { GettingStarted } from './getting-started'

const key: APIKey = { id: 'k1', provider: 'zai', label: '', last4: 'ab12', created_at: '2026-10-13T00:00:00Z' }

function book(id: string, created: string, completed = 0): Book {
  return {
    id,
    prompt: 'Truyện ' + id,
    status: 'running',
    stop_reason: '',
    provider: 'zai',
    model: 'glm-4.7',
    api_key_id: 'k1',
    budget_usd: 1,
    subscription: false,
    advance_mode: '',
    pending_text: '',
    last_error: '',
    snapshot: completed > 0 ? ({ completed_count: completed } as BookSnapshot) : undefined,
    created_at: created,
    updated_at: created,
  }
}

function step(id: string): HTMLElement {
  const el = document.querySelector<HTMLElement>(`[data-step="${id}"]`)
  if (!el) throw new Error('không thấy bước ' + id)
  return el
}

const isDone = (id: string) => within(step(id)).queryByText('(Đã xong)', { exact: false }) !== null

describe('GettingStarted', () => {
  it('chưa có gì: bước 1 là bước hiện tại, chưa bước nào xong', async () => {
    await renderWithProviders(<GettingStarted keys={[]} books={[]} />)
    expect(screen.getByRole('heading', { name: 'Bắt đầu trong 3 bước' })).toBeInTheDocument()
    expect(screen.getByText('Đã xong 0/3 bước')).toBeInTheDocument()
    expect(step('key')).toHaveAttribute('aria-current', 'step')
    expect(step('key')).toHaveClass('callout')
    expect(step('book')).not.toHaveAttribute('aria-current')
    expect(['key', 'book', 'read'].some(isDone)).toBe(false)
    expect(within(step('key')).getByRole('link', { name: 'Thêm key' })).toHaveAttribute('href', '/settings/keys')
    expect(within(step('book')).getByRole('link', { name: 'Tạo truyện' })).toHaveAttribute('href', '/books/new')
    expect(within(step('read')).queryByRole('link')).toBeNull()
  })

  it('có key: bước 1 tick, bước 2 là bước hiện tại', async () => {
    await renderWithProviders(<GettingStarted keys={[key]} books={[]} />)
    expect(screen.getByText('Đã xong 1/3 bước')).toBeInTheDocument()
    expect(isDone('key')).toBe(true)
    expect(within(step('key')).queryByRole('link')).toBeNull()
    expect(step('book')).toHaveAttribute('aria-current', 'step')
    expect(isDone('book')).toBe(false)
  })

  it('có key và truyện chưa có chương: bước 3 dẫn tới truyện mới nhất', async () => {
    const books = [book('old', '2026-10-01T00:00:00Z'), book('new', '2026-10-12T00:00:00Z')]
    await renderWithProviders(<GettingStarted keys={[key]} books={books} />)
    expect(screen.getByText('Đã xong 2/3 bước')).toBeInTheDocument()
    expect(step('read')).toHaveAttribute('aria-current', 'step')
    expect(within(step('read')).getByRole('link', { name: 'Xem truyện' })).toHaveAttribute('href', '/books/new')
  })

  it('xong cả 3 bước thì ẩn hẳn', async () => {
    const { container } = await renderWithProviders(
      <GettingStarted keys={[key]} books={[book('b1', '2026-10-12T00:00:00Z', 1)]} />,
    )
    expect(container.querySelector('section')).toBeNull()
    expect(screen.queryByText('Bắt đầu trong 3 bước')).toBeNull()
  })

  it('có truyện có chương nhưng đã xoá hết key: vẫn hiện, bước 1 là bước hiện tại', async () => {
    await renderWithProviders(<GettingStarted keys={[]} books={[book('b1', '2026-10-12T00:00:00Z', 2)]} />)
    expect(step('key')).toHaveAttribute('aria-current', 'step')
    expect(isDone('book')).toBe(true)
    expect(isDone('read')).toBe(true)
  })
})
