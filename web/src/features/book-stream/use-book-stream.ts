import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useState, useSyncExternalStore } from 'react'
import { connectBookStream } from './connect'
import { createBookStreamStore, type BookStreamState, type BookStreamStore } from './stream-store'

/** Gọi trong component có `key={bookId}` để mỗi truyện có store riêng. Không tự đăng ký nhận cập nhật. */
export function useBookStreamStore(id: string): BookStreamStore {
  const qc = useQueryClient()
  const [store] = useState(() => createBookStreamStore())
  useEffect(() => connectBookStream(id, store, qc), [id, store, qc])
  return store
}

/** Chỉ render lại khi giá trị chọn ra đổi (so sánh Object.is). */
export function useStreamSelector<T>(store: BookStreamStore, select: (s: BookStreamState) => T): T {
  return useSyncExternalStore(store.subscribe, () => select(store.getSnapshot()))
}

export const selectEvents = (s: BookStreamState) => s.events
export const selectStreamText = (s: BookStreamState) => s.streamText
