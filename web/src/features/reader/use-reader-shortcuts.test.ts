import { fireEvent, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useReaderShortcuts } from './use-reader-shortcuts'

function setup(over: { disabled?: boolean; withPrev?: boolean } = {}) {
  const onPrev = vi.fn()
  const onNext = vi.fn()
  const onExit = vi.fn()
  const hook = renderHook((p: { disabled?: boolean }) =>
    useReaderShortcuts({ onPrev: over.withPrev === false ? undefined : onPrev, onNext, onExit, disabled: p.disabled }),
    { initialProps: { disabled: over.disabled } },
  )
  return { onPrev, onNext, onExit, hook }
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('useReaderShortcuts', () => {
  it('← / → / Esc gọi đúng hành động', () => {
    const { onPrev, onNext, onExit } = setup()
    fireEvent.keyDown(window, { key: 'ArrowLeft' })
    fireEvent.keyDown(window, { key: 'ArrowRight' })
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onPrev).toHaveBeenCalledTimes(1)
    expect(onNext).toHaveBeenCalledTimes(1)
    expect(onExit).toHaveBeenCalledTimes(1)
  })

  it('bỏ qua khi có modifier hoặc phím khác', () => {
    const { onPrev, onNext, onExit } = setup()
    fireEvent.keyDown(window, { key: 'ArrowLeft', altKey: true })
    fireEvent.keyDown(window, { key: 'ArrowRight', ctrlKey: true })
    fireEvent.keyDown(window, { key: 'ArrowRight', metaKey: true })
    fireEvent.keyDown(window, { key: 'ArrowLeft', shiftKey: true })
    fireEvent.keyDown(window, { key: 'a' })
    expect(onPrev).not.toHaveBeenCalled()
    expect(onNext).not.toHaveBeenCalled()
    expect(onExit).not.toHaveBeenCalled()
  })

  it('không bắt phím khi đang gõ trong input/textarea/contenteditable', () => {
    const { onPrev, onNext, onExit } = setup()
    const input = document.createElement('input')
    const area = document.createElement('textarea')
    const editable = document.createElement('div')
    editable.setAttribute('contenteditable', 'true')
    document.body.append(input, area, editable)
    fireEvent.keyDown(input, { key: 'ArrowLeft' })
    fireEvent.keyDown(area, { key: 'ArrowRight' })
    fireEvent.keyDown(editable, { key: 'Escape' })
    expect(onPrev).not.toHaveBeenCalled()
    expect(onNext).not.toHaveBeenCalled()
    expect(onExit).not.toHaveBeenCalled()
  })

  it('không bắt phím trong dialog hoặc khi bị tắt (bảng cài đặt đang mở)', () => {
    const { onNext, onExit, hook } = setup({ disabled: true })
    fireEvent.keyDown(window, { key: 'ArrowRight' })
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onNext).not.toHaveBeenCalled()
    expect(onExit).not.toHaveBeenCalled()

    hook.rerender({ disabled: false })
    const dialog = document.createElement('div')
    dialog.setAttribute('role', 'dialog')
    const btn = document.createElement('button')
    dialog.append(btn)
    document.body.append(dialog)
    fireEvent.keyDown(btn, { key: 'Escape' })
    expect(onExit).not.toHaveBeenCalled()

    fireEvent.keyDown(window, { key: 'ArrowRight' })
    expect(onNext).toHaveBeenCalledTimes(1)
  })

  it('không có chương trước thì ← không làm gì và không chặn mặc định', () => {
    const { onPrev } = setup({ withPrev: false })
    const ev = new KeyboardEvent('keydown', { key: 'ArrowLeft', cancelable: true })
    window.dispatchEvent(ev)
    expect(onPrev).not.toHaveBeenCalled()
    expect(ev.defaultPrevented).toBe(false)
  })

  it('gỡ listener khi unmount', () => {
    const { onExit, hook } = setup()
    hook.unmount()
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onExit).not.toHaveBeenCalled()
  })
})
