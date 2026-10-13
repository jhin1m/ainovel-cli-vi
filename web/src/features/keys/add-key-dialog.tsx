import { Dialog } from '@/components/ui/dialog'
import { msg } from '@/lib/messages'
import { AddKeyForm } from './add-key-form'

export function AddKeyDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  return (
    <Dialog open={open} title={msg.keys.addTitle} onClose={onClose} size="lg">
      {/* Mount lại mỗi lần mở để xoá sạch key đã nhập. Danh sách key tự làm mới khi lưu. */}
      <AddKeyForm onSaved={() => {}} onClose={onClose} />
    </Dialog>
  )
}
