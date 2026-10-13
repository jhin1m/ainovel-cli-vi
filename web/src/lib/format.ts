const LOCALE = 'vi-VN'

const numberFmt = new Intl.NumberFormat(LOCALE)
const dateFmt = new Intl.DateTimeFormat(LOCALE, { day: '2-digit', month: '2-digit', year: 'numeric' })

export function formatNumber(n: number): string {
  return numberFmt.format(Number.isFinite(n) ? n : 0)
}

/** Tiền USD, ví dụ "1,82 US$". */
export function formatUSD(n: number, fractionDigits = 2): string {
  const v = Number.isFinite(n) ? n : 0
  return (
    new Intl.NumberFormat(LOCALE, {
      minimumFractionDigits: fractionDigits,
      maximumFractionDigits: fractionDigits,
    }).format(v) + ' US$'
  )
}

/** Số USD không kèm đơn vị, để ghép dạng "1,82 / 5 US$". */
export function formatUSDPlain(n: number, fractionDigits = 2): string {
  return new Intl.NumberFormat(LOCALE, {
    minimumFractionDigits: fractionDigits,
    maximumFractionDigits: fractionDigits,
  }).format(Number.isFinite(n) ? n : 0)
}

export function formatDate(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '' : dateFmt.format(d)
}

/** Giá model: USD cho 1 triệu token. */
export function formatPer1M(n: number): string {
  return `${formatUSDPlain(n, n < 1 ? 3 : 2)} US$`
}
