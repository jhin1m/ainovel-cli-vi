import { useEffect, useId, useRef, useState } from 'react'
import { inputClass } from '@/components/ui/field'
import { msg } from '@/lib/messages'
import { MAX_PROMPT_RUNES, clampPrompt, runeLength, type PromptOptions, type StoryLength } from './new-book-logic'
import { PROMPT_PRESETS } from './prompt-presets'

/** Thời gian hiện nút "Hoàn tác" sau khi gợi ý thay chữ đã gõ. */
export const UNDO_MS = 5000

interface Props {
  idea: string
  onIdea: (v: string) => void
  options: PromptOptions
  onOptions: (v: PromptOptions) => void
  /** Nội dung đã ghép gửi cho AI. */
  composed: string
  error?: string
}

const sectionTitle = 'm-0 text-sm font-semibold tracking-wider text-muted uppercase'

/** Cột trái của form tạo truyện: ý tưởng, gợi ý, tuỳ chỉnh thêm và xem trước. */
export function PromptSection({ idea, onIdea, options, onOptions, composed, error }: Props) {
  const ids = useId()
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const undoRef = useRef<HTMLButtonElement>(null)
  // prev: chữ của người dùng trước khi bấm gợi ý; applied: nội dung gợi ý vừa điền.
  const [undo, setUndo] = useState<{ prev: string; applied: string; label: string } | null>(null)
  const [restored, setRestored] = useState(false)

  useEffect(() => {
    if (!undo) return
    const t = setTimeout(() => {
      // Nút sắp biến mất: đừng để focus bàn phím rơi về body.
      if (document.activeElement === undoRef.current) textareaRef.current?.focus()
      setUndo(null)
    }, UNDO_MS)
    return () => clearTimeout(t)
  }, [undo])

  function applyPreset(label: string, text: string) {
    // Bấm lại đúng gợi ý đang hiện (hoặc double-click): không làm gì, giữ nút hoàn tác.
    if (idea === text) return
    setRestored(false)
    if (undo && idea === undo.applied) {
      // Đổi từ gợi ý này sang gợi ý khác: hoàn tác vẫn trả về chữ gốc của người dùng.
      setUndo({ ...undo, applied: text, label })
    } else {
      // Ô trống thì điền thẳng; có chữ thì thay và cho hoàn tác (không dùng confirm()).
      setUndo(idea.trim() === '' ? null : { prev: idea, applied: text, label })
    }
    onIdea(text)
  }

  function restore() {
    if (!undo) return
    onIdea(undo.prev)
    setUndo(null)
    setRestored(true)
    // Nút hoàn tác biến mất: đưa focus về ô ý tưởng thay vì rơi về body.
    textareaRef.current?.focus()
  }

  const set = <K extends keyof PromptOptions>(k: K, v: PromptOptions[K]) => onOptions({ ...options, [k]: v })
  const hintId = `${ids}-hint`
  const errId = `${ids}-err`
  const ideasId = `${ids}-ideas`

  return (
    <div className="flex min-w-0 flex-col gap-5">
      <div className="flex flex-col gap-2.5">
        <label htmlFor="prompt" className="font-serif text-[1.75rem] leading-tight font-semibold tracking-tight">
          {msg.newBook.heading}
        </label>
        <textarea
          ref={textareaRef}
          id="prompt"
          rows={5}
          value={idea}
          onChange={(e) => {
            setRestored(false)
            onIdea(clampPrompt(e.target.value))
          }}
          placeholder={msg.newBook.placeholder}
          aria-invalid={error ? true : undefined}
          aria-describedby={[hintId, error ? errId : ''].filter(Boolean).join(' ')}
          className="box-border w-full rounded-card border border-line-input bg-surface px-4 py-3.5 font-serif text-[1.0625rem] leading-relaxed text-fg"
        />
        <div id={hintId} className="flex justify-between gap-3 text-sm text-muted">
          <span>{msg.newBook.hint}</span>
          <span className="shrink-0 tabular-nums">
            {runeLength(composed)}/{MAX_PROMPT_RUNES}
          </span>
        </div>
        {error && (
          <span id={errId} role="alert" className="text-sm text-st-error">
            {error}
          </span>
        )}
      </div>

      <div className="flex flex-col gap-2.5">
        <h2 id={ideasId} className={sectionTitle}>
          {msg.newBook.ideasTitle}
        </h2>
        <div role="group" aria-labelledby={ideasId} className="flex flex-wrap gap-2">
          {PROMPT_PRESETS.map((p) => (
            <button
              key={p.label}
              type="button"
              onClick={() => applyPreset(p.label, p.text)}
              className="min-h-11 cursor-pointer rounded-full border border-line bg-surface px-3.5 text-sm text-fg hover:bg-track"
            >
              {p.label}
            </button>
          ))}
        </div>
        <div role="status" className="text-sm">
          {undo ? (
            <span className="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-control bg-warm-bg px-3 py-1.5 text-warm">
              {msg.newBook.replaced(undo.label)}
              <button
                ref={undoRef}
                type="button"
                onClick={restore}
                className="min-h-11 cursor-pointer border-0 bg-transparent px-1 font-semibold text-warm underline"
              >
                {msg.newBook.undo}
              </button>
            </span>
          ) : restored ? (
            <span className="text-muted">{msg.newBook.restored}</span>
          ) : null}
        </div>
      </div>

      <details className="rounded-card border border-line bg-surface shadow-card">
        <summary className="flex min-h-11 cursor-pointer items-center px-4 font-medium">{msg.newBook.more}</summary>
        <div className="flex flex-col gap-4 border-t border-line px-4 pt-3 pb-4">
          <p className="m-0 text-sm text-muted">{msg.newBook.moreHint}</p>
          <ChipGroup
            label={msg.newBook.genre}
            options={msg.newBook.genres.map((g) => ({ value: g, label: g }))}
            value={options.genre}
            onChange={(v) => set('genre', v)}
          />
          <ChipGroup
            label={msg.newBook.length}
            options={(Object.keys(msg.newBook.lengths) as StoryLength[]).map((k) => ({
              value: k,
              label: `${msg.newBook.lengths[k].label} (${msg.newBook.lengths[k].detail})`,
            }))}
            value={options.length}
            onChange={(v) => set('length', v as StoryLength | '')}
          />
          <ChipGroup
            label={msg.newBook.tone}
            options={msg.newBook.tones.map((t) => ({ value: t, label: t }))}
            value={options.tone}
            onChange={(v) => set('tone', v)}
          />
          <div className="grid gap-4 sm:grid-cols-2">
            <TextOption
              id={`${ids}-protagonist`}
              label={msg.newBook.protagonist}
              placeholder={msg.newBook.protagonistPlaceholder}
              value={options.protagonist}
              onChange={(v) => set('protagonist', v)}
            />
            <TextOption
              id={`${ids}-setting`}
              label={msg.newBook.setting}
              placeholder={msg.newBook.settingPlaceholder}
              value={options.setting}
              onChange={(v) => set('setting', v)}
            />
          </div>
        </div>
      </details>

      <details className="rounded-card border border-line bg-surface shadow-card">
        <summary className="flex min-h-11 cursor-pointer items-center px-4 font-medium">{msg.newBook.preview}</summary>
        <div
          data-testid="prompt-preview"
          className="border-t border-line px-4 py-3 font-serif text-base leading-relaxed break-words whitespace-pre-wrap text-fg"
        >
          {composed || <span className="font-sans text-sm text-muted">{msg.newBook.previewEmpty}</span>}
        </div>
      </details>
    </div>
  )
}

/** Nhóm chip chọn một (bấm lại để bỏ chọn). */
function ChipGroup({
  label,
  options,
  value,
  onChange,
}: {
  label: string
  options: { value: string; label: string }[]
  value: string
  onChange: (v: string) => void
}) {
  const id = useId()
  return (
    <div className="flex flex-col gap-2">
      <span id={id} className="text-sm font-medium">
        {label}
      </span>
      <div role="group" aria-labelledby={id} className="flex flex-wrap gap-2">
        {options.map((o) => {
          const on = o.value === value
          return (
            <button
              key={o.value}
              type="button"
              aria-pressed={on}
              onClick={() => onChange(on ? '' : o.value)}
              className={`min-h-11 cursor-pointer rounded-full border px-3.5 text-sm ${
                on ? 'border-accent bg-accent text-on-accent' : 'border-line bg-surface text-fg hover:bg-track'
              }`}
            >
              {o.label}
            </button>
          )
        })}
      </div>
    </div>
  )
}

function TextOption({
  id,
  label,
  placeholder,
  value,
  onChange,
}: {
  id: string
  label: string
  placeholder: string
  value: string
  onChange: (v: string) => void
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      <input
        id={id}
        maxLength={200}
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        // Enter ở ô phụ không được tạo truyện (tạo truyện là bắt đầu tốn tiền).
        onKeyDown={(e) => {
          if (e.key === 'Enter') e.preventDefault()
        }}
        className={inputClass}
      />
    </div>
  )
}
