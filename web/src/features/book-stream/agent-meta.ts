import { msg } from '@/lib/messages'

export interface AgentMeta {
  label: string
  /** Lớp Tailwind viết nguyên chuỗi để bộ quét không bỏ sót. */
  tone: string
}

export function agentMeta(agent: string | undefined): AgentMeta {
  const a = (agent ?? '').toLowerCase()
  if (a.includes('architect')) return { label: msg.book.agents.architect, tone: 'bg-ag-architect-bg text-ag-architect' }
  if (a.includes('writer')) return { label: msg.book.agents.writer, tone: 'bg-ag-writer-bg text-ag-writer' }
  if (a.includes('editor')) return { label: msg.book.agents.editor, tone: 'bg-ag-editor-bg text-ag-editor' }
  if (a === 'user' || a === 'human') return { label: msg.book.agents.user, tone: 'bg-ag-user-bg text-ag-user' }
  return { label: msg.book.agents.system, tone: 'bg-st-idle-bg text-st-idle' }
}
