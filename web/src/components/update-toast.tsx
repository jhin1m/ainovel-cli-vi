import { msg } from '@/lib/messages'
import { updateStore, useUpdateAvailable } from '@/lib/pwa-update'
import { Button } from './ui/button'

export function UpdateToast() {
  const available = useUpdateAvailable()
  if (!available) return null
  return (
    <div
      role="status"
      className="fixed inset-x-4 bottom-4 z-50 mx-auto flex max-w-md items-center justify-between gap-3 rounded-lg border border-line bg-surface p-3 shadow-lg"
    >
      <span>{msg.update.available}</span>
      <div className="flex shrink-0 gap-2">
        <Button variant="ghost" onClick={() => updateStore.dismiss()}>
          {msg.update.later}
        </Button>
        <Button onClick={() => updateStore.apply()}>{msg.update.reload}</Button>
      </div>
    </div>
  )
}
