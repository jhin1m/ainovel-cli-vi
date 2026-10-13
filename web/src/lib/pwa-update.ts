import { useSyncExternalStore } from 'react'

type Listener = () => void

/** Trạng thái "có phiên bản mới": giữ hàm kích hoạt service worker đang chờ. */
export function createUpdateStore() {
  let apply: (() => void) | null = null
  const listeners = new Set<Listener>()
  const emit = () => listeners.forEach((l) => l())
  return {
    subscribe(l: Listener) {
      listeners.add(l)
      return () => {
        listeners.delete(l)
      }
    },
    available: () => apply !== null,
    offer(fn: () => void) {
      apply = fn
      emit()
    },
    dismiss() {
      apply = null
      emit()
    },
    apply() {
      const fn = apply
      apply = null
      emit()
      fn?.()
    },
  }
}

export const updateStore = createUpdateStore()

export function useUpdateAvailable(): boolean {
  return useSyncExternalStore(updateStore.subscribe, updateStore.available)
}

/** Đăng ký service worker; chỉ chạy ở bản build (dev không có sw). */
export async function registerServiceWorker(): Promise<void> {
  if (!import.meta.env.PROD || !('serviceWorker' in navigator)) return
  try {
    const { registerSW } = await import('virtual:pwa-register')
    const update = registerSW({
      onNeedRefresh: () => updateStore.offer(() => void update(true)),
    })
  } catch (err) {
    console.warn('service worker registration failed', err)
  }
}
