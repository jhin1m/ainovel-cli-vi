import { useEffect, useState, type RefObject } from 'react'

const WIDE = '(min-width: 80rem)'
/** Chiều cao tối thiểu (px) cho hai cột của tab "Đang viết", tính cả lề dưới trang. */
const MIN_COLUMNS_PX = 18 * 16 + 24

/**
 * Bật bố cục 2 cột cao đúng một màn hình (mỗi cột tự cuộn, trang không cuộn) khi màn ≥1280px
 * và phần dưới `chromeRef` (tiêu đề, số liệu, banner, stepper, thanh tab) còn đủ chỗ cho hai cột.
 * Không đủ (laptop thấp có banner…) thì trả false để trang dùng bố cục xếp dọc và cuộn bình thường,
 * tránh cảnh vừa cuộn khung trang vừa cuộn từng cột.
 */
export function useColumnsLayout(chromeRef: RefObject<HTMLElement | null>, enabled: boolean): boolean {
  const [fits, setFits] = useState(false)

  useEffect(() => {
    const el = chromeRef.current
    if (!enabled || !el || typeof window.matchMedia !== 'function') return
    const wide = window.matchMedia(WIDE)
    const check = () => {
      // Đo theo toạ độ tài liệu để kết quả không phụ thuộc vị trí cuộn hiện tại.
      const bottom = el.getBoundingClientRect().bottom + window.scrollY
      setFits(wide.matches && window.innerHeight - bottom >= MIN_COLUMNS_PX)
    }
    const frame = requestAnimationFrame(check)
    wide.addEventListener('change', check)
    window.addEventListener('resize', check)
    // Banner xuất hiện/biến mất làm phần đầu trang đổi chiều cao.
    const ro = typeof ResizeObserver === 'function' ? new ResizeObserver(check) : null
    ro?.observe(el)
    return () => {
      cancelAnimationFrame(frame)
      wide.removeEventListener('change', check)
      window.removeEventListener('resize', check)
      ro?.disconnect()
    }
  }, [chromeRef, enabled])

  return enabled && fits
}
