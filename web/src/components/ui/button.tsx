import type { ButtonHTMLAttributes } from 'react'

type Variant = 'primary' | 'secondary' | 'danger' | 'ghost'

const base =
  'inline-flex items-center justify-center gap-2 min-h-[44px] px-5 rounded-control text-base font-semibold cursor-pointer transition-colors disabled:opacity-60 disabled:cursor-not-allowed'

const variants: Record<Variant, string> = {
  primary: 'bg-accent text-on-accent hover:bg-accent-hover',
  secondary: 'bg-surface text-fg border border-line hover:bg-track',
  danger: 'bg-st-error text-white hover:opacity-90',
  ghost: 'bg-transparent text-fg hover:bg-track',
}

export function buttonClass(variant: Variant = 'primary', extra = ''): string {
  return `${base} ${variants[variant]} ${extra}`.trim()
}

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
}

export function Button({ variant = 'primary', className = '', type = 'button', ...rest }: Props) {
  return <button type={type} className={buttonClass(variant, className)} {...rest} />
}
