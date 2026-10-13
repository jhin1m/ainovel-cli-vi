import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { BookStatus } from '@/api/types.gen'
import { BOOK_STATUSES } from '@/api/status'
import { StatusBadge } from './status-badge'

const labels: Record<string, string> = {
  queued: 'Xếp hàng',
  running: 'Đang viết',
  waiting_review: 'Chờ duyệt',
  paused: 'Tạm dừng',
  budget_exhausted: 'Hết ngân sách',
  completed: 'Hoàn thành',
  failed: 'Lỗi',
  interrupted: 'Bị gián đoạn',
  deleting: 'Đang xoá',
}

describe('StatusBadge', () => {
  it.each(BOOK_STATUSES)('hiện nhãn tiếng Việt cho %s', (s) => {
    render(<StatusBadge status={s} />)
    expect(screen.getByText(labels[s]!)).toBeInTheDocument()
  })

  it('trạng thái lạ không làm vỡ giao diện', () => {
    render(<StatusBadge status={'xyz' as BookStatus} />)
    expect(screen.getByText('Không rõ')).toBeInTheDocument()
  })
})
