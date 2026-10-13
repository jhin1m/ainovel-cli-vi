import { Check, Copy, Terminal } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { msg } from '@/lib/messages'

// Lệnh lấy đúng từ README.md mục "## Cài đặt". Bản Việt hoá chỉ cài từ mã nguồn:
// script cài, `go install` và image đều trỏ về bản upstream tiếng Trung.
export const REPO_URL = 'https://github.com/jhin1m/ainovel-cli-vi'
export const INSTALL_CMD = [
  'git clone https://github.com/jhin1m/ainovel-cli-vi.git',
  'cd ainovel-cli-vi',
  'go build -o ainovel-cli ./cmd/ainovel-cli',
  './ainovel-cli --version',
].join('\n')
export const RUN_CMD = 'ainovel-cli'

type CopyState = 'idle' | 'copied' | 'failed'

function CodeBlock({ label, code }: { label: string; code: string }) {
  const [state, setState] = useState<CopyState>('idle')
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined)
  useEffect(() => () => clearTimeout(timer.current), [])
  const t = msg.onboarding.runLocally

  async function copy() {
    try {
      await navigator.clipboard.writeText(code)
      setState('copied')
    } catch {
      // Không có quyền clipboard hoặc trang không an toàn (http): để user tự chọn chép.
      setState('failed')
    }
    clearTimeout(timer.current)
    timer.current = setTimeout(() => setState('idle'), 2500)
  }

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm font-semibold">{label}</span>
        <button
          type="button"
          onClick={() => void copy()}
          aria-label={`${t.copy}: ${label}`}
          className="inline-flex min-h-11 cursor-pointer items-center gap-1.5 rounded-control px-3 text-sm font-medium text-accent hover:bg-track"
        >
          {state === 'copied' ? <Check aria-hidden size={16} /> : <Copy aria-hidden size={16} />}
          {state === 'copied' ? t.copied : t.copy}
        </button>
      </div>
      <pre className="m-0 overflow-x-auto rounded-control bg-track p-3 font-mono text-sm leading-relaxed">
        <code>{code}</code>
      </pre>
      {/* Báo kết quả chép cho screen reader; chỉ hiện chữ khi chép lỗi. */}
      <span role="status" className={state === 'failed' ? 'text-sm text-st-error' : 'sr-only'}>
        {state === 'failed' ? t.copyFailed : state === 'copied' ? t.copied : ''}
      </span>
    </div>
  )
}

/** Thẻ "Chạy trên máy bạn": so sánh web với CLI và lệnh cài lấy từ README. Không gọi backend. */
export function RunLocally() {
  const t = msg.onboarding.runLocally
  return (
    <details className="group rounded-card border border-line-card bg-surface shadow-card">
      <summary className="flex min-h-11 cursor-pointer list-none items-center gap-3 rounded-card p-4 [&::-webkit-details-marker]:hidden">
        <Terminal aria-hidden size={20} className="shrink-0 text-muted" />
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="font-semibold">{t.title}</span>
          <span className="text-sm text-muted">{t.summary}</span>
        </span>
        <span aria-hidden className="text-muted transition-transform group-open:rotate-90 motion-reduce:transition-none">
          ›
        </span>
      </summary>
      <div className="flex flex-col gap-4 border-t border-line px-4 pt-3 pb-4">
        <ul className="m-0 flex flex-col gap-1.5 pl-5 text-sm leading-relaxed">
          <li>{t.web}</li>
          <li>{t.cli}</li>
        </ul>
        <CodeBlock label={t.install} code={INSTALL_CMD} />
        <CodeBlock label={t.run} code={RUN_CMD} />
        <p className="m-0 text-sm text-muted">{t.runHint}</p>
        <a href={REPO_URL + '#readme'} target="_blank" rel="noopener noreferrer" className="text-sm font-medium">
          {t.readme} ↗<span className="sr-only"> {t.newTab}</span>
        </a>
      </div>
    </details>
  )
}
