import { act, render, renderHook, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { AgentSnapshot, Book, BookSnapshot } from '@/api/types.gen'
import { activeAgent, agentActivity, ProgressStepper, STEP_COUNT, stepFromSnapshot, useThrottled } from './progress-stepper'

function snap(over: Partial<BookSnapshot> = {}): BookSnapshot {
  return {
    phase: 'writing',
    flow: 'writing',
    completed_count: 2,
    total_chapters: 40,
    in_progress_chapter: 0,
    agents: [],
    ...over,
  } as BookSnapshot
}

function agent(name: string, state = 'working', summary = 'engine → ' + name): AgentSnapshot {
  return { name, state, summary, tool: '' }
}

/** `s = null` nghĩa là truyện chưa có snapshot. */
function book(status: string, s: BookSnapshot | null = snap()): Book {
  return { id: 'b1', status, snapshot: s ?? undefined, advance_mode: '' } as Book
}

describe('stepFromSnapshot', () => {
  it('chưa có snapshot → lập dàn ý', () => {
    expect(stepFromSnapshot(undefined, 'queued')).toEqual({ index: 0, chapter: 0 })
    expect(stepFromSnapshot(null, 'running')).toEqual({ index: 0, chapter: 0 })
  })

  it.each(['init', 'premise', 'outline', ''])('phase %j → lập dàn ý', (phase) => {
    expect(stepFromSnapshot(snap({ phase }), 'running').index).toBe(0)
  })

  it('phase writing → viết chương kế tiếp, ưu tiên chương đang viết dở', () => {
    expect(stepFromSnapshot(snap(), 'running')).toEqual({ index: 1, chapter: 3 })
    expect(stepFromSnapshot(snap({ in_progress_chapter: 5 }), 'running')).toEqual({ index: 1, chapter: 5 })
  })

  it('không vượt tổng số chương', () => {
    expect(stepFromSnapshot(snap({ completed_count: 40, total_chapters: 40 }), 'running').chapter).toBe(40)
  })

  it.each(['reviewing', 'rewriting', 'polishing'])('flow %s → biên tập chương vừa xong', (flow) => {
    expect(stepFromSnapshot(snap({ flow }), 'running')).toEqual({ index: 2, chapter: 2 })
    expect(stepFromSnapshot(snap({ flow, in_progress_chapter: 1 }), 'running')).toEqual({ index: 2, chapter: 1 })
  })

  it('editor duyệt ngay sau khi commit: chương đang biên tập là N, không phải N+1', () => {
    // Sau commit_chapter: in_progress_chapter = 0, completed_count = N, flow vẫn là writing.
    const s = snap({ completed_count: 7, in_progress_chapter: 0, agents: [agent('editor')] })
    expect(stepFromSnapshot(s, 'running')).toEqual({ index: 2, chapter: 7 })
  })

  it('agent đang chạy được ưu tiên khi truyện đang chạy', () => {
    expect(stepFromSnapshot(snap({ agents: [agent('editor')] }), 'running')).toEqual({ index: 2, chapter: 2 })
    expect(stepFromSnapshot(snap({ agents: [agent('architect_long')] }), 'running').index).toBe(0)
    expect(stepFromSnapshot(snap({ agents: [agent('writer'), agent('editor', 'idle')] }), 'running').index).toBe(1)
  })

  it('agent cũ trong snapshot bị bỏ qua khi truyện không chạy', () => {
    expect(stepFromSnapshot(snap({ agents: [agent('editor')] }), 'failed').index).toBe(1)
  })

  it('chờ duyệt → bước 4 với chương vừa xong', () => {
    expect(stepFromSnapshot(snap({ completed_count: 7 }), 'waiting_review')).toEqual({ index: 3, chapter: 7 })
  })

  it('hoàn thành → mọi bước đã qua', () => {
    expect(stepFromSnapshot(snap({ completed_count: 40 }), 'completed')).toEqual({ index: STEP_COUNT, chapter: 40 })
    expect(stepFromSnapshot(snap({ phase: 'complete' }), 'paused').index).toBe(STEP_COUNT)
  })

  it('lỗi/tạm dừng giữ bước nơi truyện dừng', () => {
    expect(stepFromSnapshot(snap({ phase: 'outline' }), 'failed').index).toBe(0)
    expect(stepFromSnapshot(snap({ flow: 'reviewing' }), 'interrupted').index).toBe(2)
    expect(stepFromSnapshot(snap(), 'paused').index).toBe(1)
  })
})

describe('agentActivity', () => {
  it('bỏ tiền tố kỹ thuật của host, đổi tên tool sang tiếng Việt', () => {
    expect(agentActivity(agent('writer', 'working', 'writer → draft_chapter'))).toBe('Viết bản nháp chương')
    expect(agentActivity(agent('writer', 'working', 'writer → tool_moi'))).toBe('tool_moi')
    expect(agentActivity(agent('writer', 'working', 'engine → writer (Viết chương 3: Gặp lại)'))).toBe('Viết chương 3: Gặp lại')
    expect(agentActivity(agent('editor', 'working', 'engine → editor'))).toBe('Đang nhận việc')
    expect(agentActivity(agent('writer', 'running', 'Đang viết chương'))).toBe('Đang viết chương')
    expect(agentActivity({ name: 'writer', state: 'working', summary: '', tool: 'commit_chapter' })).toBe('Lưu chương')
  })
})

describe('activeAgent', () => {
  it('chọn agent working, ổn định theo tên', () => {
    expect(activeAgent(snap({ agents: [agent('writer', 'idle'), agent('editor', '')] }))).toBeUndefined()
    expect(activeAgent(snap({ agents: [agent('writer', 'running')] }))?.name).toBe('writer')
    expect(activeAgent(snap({ agents: [agent('writer'), agent('editor')] }))?.name).toBe('editor')
    expect(activeAgent(undefined)).toBeUndefined()
  })
})

describe('ProgressStepper', () => {
  const current = () => screen.getAllByRole('listitem').filter((li) => li.getAttribute('aria-current') === 'step')

  it.each([
    ['queued', null, 'Lập dàn ý'],
    ['running', snap(), 'Viết chương 3'],
    ['running', snap({ flow: 'reviewing' }), 'Biên tập'],
    ['waiting_review', snap(), 'Chờ bạn duyệt'],
  ])('%s: đúng một bước aria-current', (status, s, label) => {
    render(<ProgressStepper book={book(status, s)} />)
    const cur = current()
    expect(cur).toHaveLength(1)
    expect(cur[0]).toHaveTextContent(label)
  })

  it('bước đã qua có chữ "(Đã xong)" cho screen reader', () => {
    render(<ProgressStepper book={book('waiting_review')} />)
    expect(screen.getAllByText('(Đã xong)')).toHaveLength(3)
  })

  it('chế độ tự động: không có bước "Chờ bạn duyệt"; chế độ duyệt thì có', () => {
    const { rerender } = render(<ProgressStepper book={{ ...book('running'), advance_mode: 'auto' }} />)
    expect(screen.getAllByRole('listitem')).toHaveLength(3)
    expect(screen.queryByText('Chờ bạn duyệt')).toBeNull()
    rerender(<ProgressStepper book={{ ...book('running'), advance_mode: 'review' }} />)
    expect(screen.getAllByRole('listitem')).toHaveLength(4)
    expect(screen.getByText('Chờ bạn duyệt')).toBeInTheDocument()
  })

  it('chế độ tự động nhưng đang chờ duyệt thật thì vẫn có bước chờ duyệt (là bước hiện tại)', () => {
    render(<ProgressStepper book={{ ...book('waiting_review'), advance_mode: 'auto' }} />)
    expect(current()[0]).toHaveTextContent('Chờ bạn duyệt')
  })

  it('chế độ tự động đã hoàn thành: 3 bước đều đã qua', () => {
    render(<ProgressStepper book={{ ...book('completed', snap({ phase: 'complete' })), advance_mode: 'auto' }} />)
    expect(screen.getAllByText('(Đã xong)')).toHaveLength(3)
    expect(current()).toHaveLength(0)
  })

  it('hoàn thành: không bước nào là hiện tại, có câu đã xong', () => {
    render(<ProgressStepper book={book('completed', snap({ phase: 'complete' }))} />)
    expect(current()).toHaveLength(0)
    expect(screen.getByText('Đã viết xong toàn bộ truyện.')).toBeInTheDocument()
  })

  it('dòng agent hiện tên agent + summary khi đang chạy, ẩn khi không', () => {
    const s = snap({ agents: [agent('writer', 'working', 'writer → draft_chapter')] })
    const { rerender } = render(<ProgressStepper book={book('running', s)} />)
    const section = screen.getByRole('region', { name: 'Đang làm' })
    expect(within(section).getByText('Người viết')).toBeInTheDocument()
    expect(within(section).getByText('Viết bản nháp chương')).toBeInTheDocument()

    rerender(<ProgressStepper book={book('paused', s)} />)
    expect(screen.queryByText('Viết bản nháp chương')).not.toBeInTheDocument()
  })

  it('vùng aria-live đọc bản tóm tắt', () => {
    const s = snap({ agents: [agent('editor', 'working', 'editor → save_review')] })
    render(<ProgressStepper book={book('running', s)} />)
    const live = document.querySelector('[aria-live="polite"]')
    expect(live).toHaveTextContent('Biên tập: Lưu nhận xét biên tập')
  })
})

describe('useThrottled', () => {
  afterEach(() => vi.useRealTimers())

  it('gom cập nhật dồn dập, luôn áp giá trị cuối', () => {
    vi.useFakeTimers()
    const { result, rerender } = renderHook(({ v }) => useThrottled(v, 1000), { initialProps: { v: 'a' } })
    rerender({ v: 'b' })
    act(() => vi.advanceTimersByTime(0))
    expect(result.current).toBe('b')
    rerender({ v: 'c' })
    rerender({ v: 'd' })
    act(() => vi.advanceTimersByTime(500))
    expect(result.current).toBe('b')
    act(() => vi.advanceTimersByTime(500))
    expect(result.current).toBe('d')
  })
})
