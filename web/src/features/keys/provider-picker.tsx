import { useRef, type KeyboardEvent } from 'react'
import type { ProviderInfo } from '@/api/types.gen'
import { ProviderIcon } from '@/components/provider-icon'
import { msg } from '@/lib/messages'

interface Props {
  providers: ProviderInfo[]
  value: string
  onChange: (name: string) => void
  /** id của nhãn nhóm. */
  labelledBy: string
  describedBy?: string
  invalid?: boolean
  compact?: boolean
}

/**
 * Lưới thẻ chọn provider theo mẫu radiogroup của WAI-ARIA: một điểm dừng Tab cho cả
 * nhóm (roving tabindex), mũi tên/Home/End vừa chuyển focus vừa chọn.
 */
export function ProviderPicker({ providers, value, onChange, labelledBy, describedBy, invalid, compact }: Props) {
  const refs = useRef<(HTMLButtonElement | null)[]>([])
  const selected = providers.findIndex((p) => p.name === value)
  const tabStop = selected >= 0 ? selected : 0

  function move(e: KeyboardEvent, from: number) {
    const last = providers.length - 1
    let to: number
    switch (e.key) {
      case 'ArrowRight':
      case 'ArrowDown':
        to = from === last ? 0 : from + 1
        break
      case 'ArrowLeft':
      case 'ArrowUp':
        to = from === 0 ? last : from - 1
        break
      case 'Home':
        to = 0
        break
      case 'End':
        to = last
        break
      default:
        return
    }
    e.preventDefault()
    const next = providers[to]
    if (!next) return
    onChange(next.name)
    refs.current[to]?.focus()
  }

  return (
    <div
      role="radiogroup"
      aria-labelledby={labelledBy}
      aria-describedby={describedBy}
      aria-invalid={invalid ? true : undefined}
      className={`grid grid-cols-2 md:grid-cols-3 ${compact ? 'gap-1.5' : 'gap-2'}`}
    >
      {providers.map((p, i) => {
        const checked = i === selected
        return (
          <button
            key={p.name}
            ref={(el) => {
              refs.current[i] = el
            }}
            type="button"
            role="radio"
            aria-checked={checked}
            tabIndex={i === tabStop ? 0 : -1}
            onClick={() => onChange(p.name)}
            onKeyDown={(e) => move(e, i)}
            className={`flex cursor-pointer items-center gap-2.5 rounded-control border px-3 text-left text-fg transition-colors ${
              compact ? 'min-h-11 py-2' : 'min-h-14 py-2.5'
            } ${checked ? 'border-accent bg-accent-soft ring-1 ring-accent' : 'border-line-input bg-surface hover:bg-track'}`}
          >
            <ProviderIcon name={p.name} size={compact ? 20 : 24} />
            <span className="flex min-w-0 flex-col items-start gap-0.5">
              <span className="text-sm leading-tight font-medium break-words">{p.label}</span>
              {p.subscription && (
                <span className="rounded-pill bg-warm-bg px-1.5 py-px text-xs font-semibold text-warm">
                  {msg.keys.subscription}
                </span>
              )}
            </span>
          </button>
        )
      })}
    </div>
  )
}
