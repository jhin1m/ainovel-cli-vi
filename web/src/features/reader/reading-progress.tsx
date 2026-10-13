import { useEffect, useState } from 'react'
import { msg } from '@/lib/messages'

/** Phần trăm đã cuộn của trang (0–100). Trang không cuộn được thì coi như đã đọc hết. */
export function scrollPercent(scrollY: number, scrollHeight: number, viewport: number): number {
  const max = scrollHeight - viewport
  if (max <= 0) return 100
  return Math.min(100, Math.max(0, Math.round((scrollY / max) * 100)))
}

/**
 * Theo dõi tiến độ cuộn của cửa sổ. Đo lại cả khi chiều cao trang đổi mà không cuộn
 * (chương mới tải xong, đổi cỡ chữ, khối cuối chương hiện ra).
 */
export function useScrollProgress(): number {
  const [pct, setPct] = useState(0)

  useEffect(() => {
    let frame = 0
    const measure = () => {
      frame = 0
      const el = document.documentElement
      setPct(scrollPercent(window.scrollY, el.scrollHeight, window.innerHeight))
    }
    // Gộp các sự kiện cuộn vào một lần đo mỗi khung hình.
    const schedule = () => {
      if (!frame) frame = requestAnimationFrame(measure)
    }
    measure()
    window.addEventListener('scroll', schedule, { passive: true })
    window.addEventListener('resize', schedule)
    const ro = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(schedule)
    ro?.observe(document.body)
    return () => {
      if (frame) cancelAnimationFrame(frame)
      ro?.disconnect()
      window.removeEventListener('scroll', schedule)
      window.removeEventListener('resize', schedule)
    }
  }, [])

  return pct
}

/**
 * Thanh mỏng ở đáy header reader cho biết đã đọc bao nhiêu phần chương.
 * Tự giữ state tiến độ để khi cuộn chỉ thanh này render lại, không render lại cả chương.
 */
export function ReadingProgress() {
  const value = useScrollProgress()
  return (
    <div
      role="progressbar"
      aria-label={msg.reader.progress}
      aria-valuemin={0}
      aria-valuemax={100}
      // Báo theo bước 5% để trình đọc màn hình không thông báo liên tục khi cuộn.
      aria-valuenow={Math.round(value / 5) * 5}
      className="pointer-events-none absolute inset-x-0 bottom-0 h-[3px] overflow-hidden"
    >
      <div
        className="h-full origin-left bg-current opacity-60 transition-transform duration-150 motion-reduce:transition-none"
        style={{ transform: `scaleX(${value / 100})` }}
      />
    </div>
  )
}
