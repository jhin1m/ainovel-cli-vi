// Chương đọc gần nhất của từng truyện, lưu trong localStorage của trình duyệt này.
// Chỉ để tiện (thẻ truyện hiện "Đọc tiếp chương N"); mất dữ liệu thì thẻ chỉ không hiện nút đó.

const prefix = 'ainovel:last-read:'

export function lastReadKey(bookId: string): string {
  return prefix + bookId
}

/** Số chương đọc gần nhất, `null` nếu chưa có, giá trị hỏng hoặc không đọc được storage. */
export function getLastRead(bookId: string): number | null {
  try {
    const raw = localStorage.getItem(lastReadKey(bookId))
    if (raw === null) return null
    const n = Number(raw)
    return Number.isInteger(n) && n > 0 ? n : null
  } catch {
    return null
  }
}

/** Ghi chương đang đọc. Bỏ qua số không hợp lệ và lỗi storage (chế độ riêng tư, hết dung lượng). */
export function setLastRead(bookId: string, n: number): void {
  if (!Number.isInteger(n) || n <= 0) return
  try {
    localStorage.setItem(lastReadKey(bookId), String(n))
  } catch {
    // Không lưu được thì thôi, không ảnh hưởng việc đọc.
  }
}
