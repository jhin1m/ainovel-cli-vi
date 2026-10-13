interface Props {
  /** Phần trăm 0..100; giá trị ngoài khoảng bị kẹp lại. */
  value: number
  /** Cạnh vòng, px. Mặc định 44. */
  size?: number
  /** Độ dày nét, px. Mặc định 4. */
  stroke?: number
  /** Lớp màu cho nét tiến độ (dùng `text-*`, nét vẽ bằng `currentColor`). */
  className?: string
}

export function clampPct(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.min(100, Math.max(0, Math.round(value)))
}

/** Vòng tiến độ nhỏ, thay cho thanh ngang. Có `role="img"` và nhãn "x%" cho screen reader. */
export function ProgressRing({ value, size = 44, stroke = 4, className = 'text-accent' }: Props) {
  const pct = clampPct(value)
  const r = (size - stroke) / 2
  const c = 2 * Math.PI * r
  return (
    <svg
      role="img"
      aria-label={`${pct}%`}
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      className={`shrink-0 ${className}`}
    >
      <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--track)" strokeWidth={stroke} />
      <circle
        cx={size / 2}
        cy={size / 2}
        r={r}
        fill="none"
        stroke="currentColor"
        strokeWidth={stroke}
        strokeLinecap="round"
        strokeDasharray={c}
        strokeDashoffset={c * (1 - pct / 100)}
        transform={`rotate(-90 ${size / 2} ${size / 2})`}
        data-progress-arc
      />
      <text
        x="50%"
        y="50%"
        dominantBaseline="central"
        textAnchor="middle"
        aria-hidden
        className="fill-fg text-xs font-semibold tabular-nums"
      >
        {pct}
      </text>
    </svg>
  )
}
