import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it } from 'vitest'
import type { ProviderInfo } from '@/api/types.gen'
import { ProviderPicker } from './provider-picker'

const providers: ProviderInfo[] = [
  { name: 'openrouter', label: 'OpenRouter', subscription: false, models: [] },
  { name: 'zai-coding', label: 'Z.ai Coding Plan', subscription: true, models: [] },
  { name: 'zai', label: 'Z.ai', subscription: false, models: [] },
]

function Harness({ initial = 'openrouter' }: { initial?: string }) {
  const [value, setValue] = useState(initial)
  return (
    <>
      <span id="lbl">Nhà cung cấp</span>
      <ProviderPicker providers={providers} value={value} onChange={setValue} labelledBy="lbl" />
      <button type="button">Sau nhóm</button>
    </>
  )
}

const checked = () => screen.getByRole('radio', { checked: true })

describe('ProviderPicker', () => {
  it('là radiogroup có nhãn, chỉ một điểm dừng Tab', () => {
    render(<Harness />)
    expect(screen.getByRole('radiogroup', { name: 'Nhà cung cấp' })).toBeInTheDocument()
    const radios = screen.getAllByRole('radio')
    expect(radios).toHaveLength(3)
    expect(radios.map((r) => r.tabIndex)).toEqual([0, -1, -1])
    expect(checked()).toHaveAccessibleName('OpenRouter')
  })

  it('gắn badge "Thuê bao" cho provider thuê bao', () => {
    render(<Harness />)
    expect(screen.getByRole('radio', { name: /Z\.ai Coding Plan/ })).toHaveTextContent('Thuê bao')
    expect(screen.getByRole('radio', { name: 'Z.ai' })).not.toHaveTextContent('Thuê bao')
  })

  it('mũi tên chuyển focus và chọn, có quay vòng; Home/End tới hai đầu', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    await user.tab()
    expect(document.activeElement).toBe(checked())

    await user.keyboard('{ArrowRight}')
    expect(checked()).toHaveAccessibleName(/Z\.ai Coding Plan/)
    expect(document.activeElement).toBe(checked())

    await user.keyboard('{ArrowDown}{ArrowDown}')
    expect(checked()).toHaveAccessibleName('OpenRouter')

    await user.keyboard('{ArrowLeft}')
    expect(checked()).toHaveAccessibleName('Z.ai')
    expect(document.activeElement).toBe(checked())

    await user.keyboard('{Home}')
    expect(checked()).toHaveAccessibleName('OpenRouter')
    await user.keyboard('{End}')
    expect(checked()).toHaveAccessibleName('Z.ai')
    expect(screen.getAllByRole('radio').map((r) => r.tabIndex)).toEqual([-1, -1, 0])
  })

  it('Tab rời khỏi nhóm thay vì đi qua từng thẻ', async () => {
    const user = userEvent.setup()
    render(<Harness initial="zai-coding" />)
    await user.tab()
    expect(document.activeElement).toHaveAccessibleName(/Z\.ai Coding Plan/)
    await user.tab()
    expect(document.activeElement).toHaveAccessibleName('Sau nhóm')
    await user.tab({ shift: true })
    expect(document.activeElement).toHaveAccessibleName(/Z\.ai Coding Plan/)
  })

  it('bấm chuột chọn thẻ', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    await user.click(screen.getByRole('radio', { name: 'Z.ai' }))
    expect(checked()).toHaveAccessibleName('Z.ai')
  })
})
