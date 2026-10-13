import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useState, useSyncExternalStore } from 'react'
import { connectBookStream } from './connect'
import { createBookStreamStore, type BookStreamState } from './stream-store'

/** Gọi trong component có `key={bookId}` để mỗi truyện có store riêng. */
export function useBookStream(id: string): BookStreamState {
  const qc = useQueryClient()
  const [store] = useState(() => createBookStreamStore())
  useEffect(() => connectBookStream(id, store, qc), [id, store, qc])
  return useSyncExternalStore(store.subscribe, store.getSnapshot)
}
