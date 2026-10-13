import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { INSTALL_CMD, REPO_URL, RUN_CMD, RunLocally } from './run-locally'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('RunLocally', () => {
  it('lệnh cài và lệnh chạy khớp README', () => {
    const readme = readFileSync(resolve(__dirname, '../../../../README.md'), 'utf8')
    expect(readme).toContain(INSTALL_CMD)
    expect(readme).toContain('```bash\n' + RUN_CMD + '\n```')
    // Clone qua HTTPS: đa số user không cài khoá SSH cho GitHub.
    expect(INSTALL_CMD).toContain('git clone https://github.com/jhin1m/ainovel-cli-vi.git')
    expect(readme).not.toContain('git@github.com:jhin1m/ainovel-cli-vi.git')
    expect(REPO_URL).toBe('https://github.com/jhin1m/ainovel-cli-vi')
  })

  it('nút chép gọi clipboard với đúng lệnh', async () => {
    const user = userEvent.setup()
    const write = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue()
    render(<RunLocally />)
    await user.click(screen.getByText('Chạy trên máy bạn'))
    await user.click(screen.getByRole('button', { name: /Chép lệnh: Cài đặt/ }))
    expect(write).toHaveBeenCalledWith(INSTALL_CMD)
    expect(await screen.findByRole('button', { name: /Chép lệnh: Cài đặt/ })).toHaveTextContent('Đã chép')
    expect(screen.getAllByRole('status').some((el) => el.textContent === 'Đã chép')).toBe(true)
  })

  it('clipboard lỗi thì báo chép thủ công, không vỡ', async () => {
    const user = userEvent.setup()
    vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(new Error('denied'))
    render(<RunLocally />)
    await user.click(screen.getByText('Chạy trên máy bạn'))
    await user.click(screen.getByRole('button', { name: /Chép lệnh: Chạy/ }))
    expect(await screen.findByText(/Không chép được/)).toBeInTheDocument()
  })

  it('link README mở tab mới an toàn', () => {
    render(<RunLocally />)
    const link = screen.getByRole('link', { name: /Xem hướng dẫn đầy đủ/, hidden: true })
    expect(link).toHaveAttribute('href', REPO_URL + '#readme')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
  })
})
