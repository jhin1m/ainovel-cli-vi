import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ProviderIcon } from './provider-icon'

// Đúng allowlist của worker (internal/entry/worker/providers.go).
const allowlist = ['openrouter', 'anthropic', 'gemini', 'openai', 'deepseek', 'qwen', 'zai-coding', 'zai', 'glm', 'grok']

describe('ProviderIcon', () => {
  it.each(allowlist)('vẽ SVG thương hiệu cho %s', (name) => {
    const { container } = render(<ProviderIcon name={name} size={24} />)
    const svg = container.querySelector('svg')
    expect(svg).not.toBeNull()
    expect(svg).toHaveAttribute('aria-hidden', 'true')
    expect(svg).toHaveAttribute('data-provider-icon', name)
    expect(svg).toHaveAttribute('width', '24')
    expect(svg?.querySelector('path')?.getAttribute('d')).toMatch(/^M/)
  })

  it('Z.ai Coding Plan dùng chung logo với Z.ai', () => {
    const a = render(<ProviderIcon name="zai-coding" />).container.querySelector('path')?.getAttribute('d')
    const b = render(<ProviderIcon name="zai" />).container.querySelector('path')?.getAttribute('d')
    expect(a).toBe(b)
  })

  it('provider lạ hiện ô chữ cái đầu', () => {
    const { container } = render(<ProviderIcon name="mistral" />)
    expect(container.querySelector('svg')).toBeNull()
    const box = container.querySelector('[data-provider-fallback]')
    expect(box).toHaveAttribute('aria-hidden', 'true')
    expect(box).toHaveTextContent('m')
  })

  it('không nhầm thuộc tính của Object thành provider', () => {
    const { container } = render(<ProviderIcon name="toString" />)
    expect(container.querySelector('svg')).toBeNull()
  })
})
