import { useId, type InputHTMLAttributes, type ReactNode } from 'react'

export const inputClass =
  'w-full min-h-[44px] box-border px-3 rounded-[10px] border border-line-input bg-surface text-fg text-[16px] font-sans'

interface FieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'id'> {
  label: string
  hint?: string
  error?: string
}

export function Field({ label, hint, error, className = '', ...rest }: FieldProps) {
  const id = useId()
  const describedBy = [hint ? `${id}-hint` : '', error ? `${id}-err` : ''].filter(Boolean).join(' ')
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      <input
        id={id}
        className={`${inputClass} ${className}`}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy || undefined}
        {...rest}
      />
      {hint && (
        <span id={`${id}-hint`} className="text-[13px] text-muted">
          {hint}
        </span>
      )}
      {error && (
        <span id={`${id}-err`} role="alert" className="text-[13px] text-st-error">
          {error}
        </span>
      )}
    </div>
  )
}

export function FormError({ children }: { children: ReactNode }) {
  if (!children) return null
  return (
    <div
      role="alert"
      className="rounded-control bg-st-error-bg text-st-error px-3.5 py-3 text-sm leading-relaxed"
    >
      {children}
    </div>
  )
}
