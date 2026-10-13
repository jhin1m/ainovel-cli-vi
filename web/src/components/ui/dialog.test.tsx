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
})
