export type CoverTone = 1 | 2 | 3 | 4 | 5 | 6

/** Chọn một trong 6 tông bìa (`--cover-1..6`) theo id truyện. Cùng id luôn ra cùng tông. */
export function coverTone(id: string): CoverTone {
  // FNV-1a 32 bit: rẻ, ổn định giữa các lần tải trang, rải đều cho chuỗi ngắn.
  let h = 0x811c9dc5
  for (let i = 0; i < id.length; i++) {
    h ^= id.charCodeAt(i)
    h = Math.imul(h, 0x01000193)
  }
  return (((h >>> 0) % 6) + 1) as CoverTone
}
