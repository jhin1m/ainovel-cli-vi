import { CircleAlert, CircleCheck, LoaderCircle } from 'lucide-react'
import type { ReactNode } from 'react'
import { ApiError, errorMessage } from '@/api/client'
import { KeyCheckAuthFailed, KeyCheckPlanMismatch, type TestKeyResult } from '@/api/types.gen'
import { msg } from '@/lib/messages'

interface Props {
  pending: boolean
  result?: TestKeyResult
  error: unknown
}

/**
 * Kết quả "Kiểm tra key". Vùng `role="status"` luôn có mặt để trình đọc màn hình
 * đọc được thay đổi. Chỉ hiện `message` do server viết, không có văn bản của provider.
 */
export function KeyCheckStatus({ pending, result, error }: Props) {
  return (
    <div role="status">
      {pending ? (
        <p className="m-0 flex items-center gap-2 text-sm text-muted">
          <LoaderCircle aria-hidden size={16} className="animate-spin motion-reduce:animate-none" />
          {msg.keys.checking}
        </p>
      ) : error ? (
        <Problem>
          {error instanceof ApiError && error.status === 429 ? msg.keys.checkRateLimited : errorMessage(error)}
        </Problem>
      ) : result?.ok ? (
        <p className="m-0 flex items-start gap-2 text-sm text-accent">
          <CircleCheck aria-hidden size={16} className="mt-0.5 shrink-0" />
          {msg.keys.checkOk(result.model)}
        </p>
      ) : result ? (
        <Problem>
          <span className="block">{result.message}</span>
          {result.code && <span className="block">{msg.keys.checkCode(result.code)}</span>}
          {(result.kind === KeyCheckPlanMismatch || result.kind === KeyCheckAuthFailed) && (
            <span className="block font-medium">{msg.keys.checkWrongProvider}</span>
          )}
        </Problem>
      ) : null}
    </div>
  )
}

function Problem({ children }: { children: ReactNode }) {
  return (
    <div className="flex items-start gap-2 rounded-control bg-st-error-bg px-3 py-2.5 text-sm leading-relaxed text-st-error">
      <CircleAlert aria-hidden size={16} className="mt-0.5 shrink-0" />
      <div className="min-w-0">{children}</div>
    </div>
  )
}
