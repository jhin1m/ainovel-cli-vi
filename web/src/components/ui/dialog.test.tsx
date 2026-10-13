import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it } from 'vitest'
import { Dialog } from './dialog'

function Host() {
  const [open, setOpen] = useState(false)
  const [n, setN] = useState(0)
  return (
    <div>
      <button onClick={() => setOpen(true)}>mở</button>
      <Dialog open={open} title="Hộp" onClose={() => setOpen(false)}>
        <input aria-label="a" />
        <button>b</button>
        <button onClick={() => setN((v) => v + 1)}>tick {n}</button>
      </Dialog>
    </div>
  )
}

describe('Dialog', () => {
  it('does not steal focus when the parent re-renders with a new onClose', async () => {
    render(<Host />)
    await userEvent.click(screen.getByRole('button', { name: 'mở' }))
    await userEvent.click(screen.getByRole('button', { name: 'tick 0' }))
    const tick = screen.getByRole('button', { name: 'tick 1' })
    expect(tick).toHaveFocus()
  })

  it('traps Tab, closes on Escape and restores focus', async () => {
    render(<Host />)
    const opener = screen.getByRole('button', { name: 'mở' })
    await userEvent.click(opener)
    expect(screen.getByRole('dialog')).toHaveFocus()
    await userEvent.tab()
    expect(screen.getByLabelText('a')).toHaveFocus()
    await userEvent.tab()
    expect(screen.getByRole('button', { name: 'b' })).toHaveFocus()
    await userEvent.tab()
    expect(screen.getByRole('button', { name: 'tick 0' })).toHaveFocus()
    await userEvent.tab()
    expect(screen.getByLabelText('a')).toHaveFocus()
    await userEvent.tab({ shift: true })
    expect(screen.getByRole('button', { name: 'tick 0' })).toHaveFocus()
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(opener).toHaveFocus()
  })

  it('giữ focus trong hộp thoại khi có phần tử tabindex=-1 (radio roving, đoạn văn được focus)', async () => {
    const user = userEvent.setup()
    render(
      <div>
        <button>nền</button>
        <Dialog open title="Hộp" onClose={() => {}}>
          <button tabIndex={-1}>radio 1</button>
          <button>radio 2</button>
          <p tabIndex={-1}>đã lưu</p>
          <button>xong</button>
        </Dialog>
      </div>,
    )
    screen.getByRole('button', { name: 'radio 2' }).focus()
    await user.tab({ shift: true })
    expect(screen.getByRole('button', { name: 'xong' })).toHaveFocus()

    screen.getByText('đã lưu').focus()
    await user.tab({ shift: true })
    expect(screen.getByRole('button', { name: 'xong' })).toHaveFocus()
    screen.getByText('đã lưu').focus()
    await user.tab()
    expect(screen.getByRole('button', { name: 'radio 2' })).toHaveFocus()
  })

  it('mặc định cỡ md, nhận size lg/xl', () => {
    const { rerender } = render(
      <Dialog open title="Hộp" onClose={() => {}}>
        x
      </Dialog>,
    )
    expect(screen.getByRole('dialog')).toHaveClass('max-w-md', 'rounded-t-card')
    rerender(
      <Dialog open size="lg" title="Hộp" onClose={() => {}}>
        x
      </Dialog>,
    )
    expect(screen.getByRole('dialog')).toHaveClass('max-w-2xl')
    expect(screen.getByRole('dialog')).not.toHaveClass('max-w-md')
    rerender(
      <Dialog open size="xl" title="Hộp" onClose={() => {}}>
        x
      </Dialog>,
    )
    expect(screen.getByRole('dialog')).toHaveClass('max-w-4xl')
  })
})
