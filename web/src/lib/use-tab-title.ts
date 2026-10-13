import { useEffect } from 'react'
import { useBooks } from '@/api/queries'
import { msg } from './messages'
import { needsAttention } from './needs-attention'

/** "[(n) ]{title} · {appName}". n = số truyện cần xử lý; n = 0 thì bỏ tiền tố. */
export function formatTabTitle(title: string | undefined, attention: number): string {
  const base = title ? `${title} · ${msg.appName}` : msg.appName
  return attention > 0 ? `(${attention}) ${base}` : base
}

/**
 * Đặt tiêu đề tab. Danh sách truyện vẫn poll khi tab ẩn (`refetchIntervalInBackground`),
 * nên "(n)" cập nhật cả lúc user đang ở tab khác.
 */
export function useTabTitle(title?: string) {
  const { data } = useBooks()
  const attention = data ? needsAttention(data).length : 0

  useEffect(() => {
    document.title = formatTabTitle(title, attention)
  }, [title, attention])

  // Rời khung app (vd. đăng xuất) thì trả tiêu đề mặc định.
  useEffect(
    () => () => {
      document.title = msg.appName
    },
    [],
  )
}
