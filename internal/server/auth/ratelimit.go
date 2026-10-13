package auth

import (
	"crypto/sha256"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter là rate limiter in-memory theo khoá (IP, IP+email…). Mất khi restart:
// chấp nhận, lớp ngoài có thêm rule rate limit của Cloudflare.
type Limiter struct {
	mu      sync.Mutex
	every   rate.Limit
	burst   int
	idle    time.Duration // entry nhàn rỗi lâu hơn mức này mới bị sweep
	max     int           // trần số khoá theo dõi (maxLimiterEntries; test hạ xuống)
	entries map[string]*limiterEntry
	now     func() time.Time
}

type limiterEntry struct {
	lim  *rate.Limiter
	seen time.Time
}

// NewLimiter cho phép perMinute lần mỗi phút (burst = perMinute) cho mỗi khoá.
func NewLimiter(perMinute int) *Limiter {
	return NewLimiterPer(perMinute, time.Minute)
}

// minLimiterIdle: entry nhàn rỗi ít nhất chừng này mới bị dọn.
const minLimiterIdle = 10 * time.Minute

// NewLimiterPer cho phép n lần mỗi khoảng per cho mỗi khoá. Là token bucket
// (burst = n, hồi đều một lượt mỗi per/n), nên khoá mới dùng được tối đa gần 2n
// lần trong khoảng per đầu tiên. Entry chỉ bị sweep khi nhàn rỗi lâu hơn
// max(10 phút, per), tức đã hồi đầy lượt — limiter theo ngày không bị reset
// sau 10 phút.
func NewLimiterPer(n int, per time.Duration) *Limiter {
	return &Limiter{
		every:   rate.Limit(float64(n) / per.Seconds()),
		burst:   n,
		idle:    max(minLimiterIdle, per),
		max:     maxLimiterEntries,
		entries: make(map[string]*limiterEntry),
		now:     time.Now,
	}
}

// maxLimiterEntries: trần số khoá theo dõi. Vượt trần (bị dội khoá giả) thì từ
// chối khoá mới — đóng an toàn thay vì để bộ nhớ phình vô hạn. Riêng limiter có
// cửa sổ dài hơn 10 phút thì bỏ khoá cũ nhất thay vì từ chối: sweep không giải
// phóng được gì trong cả cửa sổ, từ chối sẽ khoá mọi khoá mới (vd. mọi IP mới
// không đăng ký được) suốt một ngày.
const maxLimiterEntries = 100_000

// Allow tiêu một lượt của khoá. false kèm thời gian nên chờ trước khi thử lại.
// Khoá được băm sha256 nên mỗi entry có kích thước cố định dù input dài.
func (l *Limiter) Allow(rawKey string) (bool, time.Duration) {
	sum := sha256.Sum256([]byte(rawKey))
	key := string(sum[:])
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e, ok := l.entries[key]
	if !ok {
		if len(l.entries) >= l.max {
			l.sweepLocked(now)
			if len(l.entries) >= l.max {
				if l.idle <= minLimiterIdle {
					return false, time.Minute
				}
				l.evictOldestLocked()
			}
		}
		e = &limiterEntry{lim: rate.NewLimiter(l.every, l.burst)}
		l.entries[key] = e
	}
	e.seen = now
	r := e.lim.ReserveN(now, 1)
	if delay := r.DelayFrom(now); delay > 0 {
		r.CancelAt(now)
		return false, delay
	}
	return true, 0
}

// sweepLocked bỏ khoá nhàn rỗi quá l.idle (đã hồi đầy lượt).
func (l *Limiter) sweepLocked(now time.Time) {
	for k, e := range l.entries {
		if now.Sub(e.seen) > l.idle {
			delete(l.entries, k)
		}
	}
}

// evictOldestLocked bỏ khoá lâu không thấy nhất (O(n), chỉ chạy khi map đầy).
func (l *Limiter) evictOldestLocked() {
	var oldest string
	var seen time.Time
	for k, e := range l.entries {
		if oldest == "" || e.seen.Before(seen) {
			oldest, seen = k, e.seen
		}
	}
	delete(l.entries, oldest)
}

// Sweep dọn định kỳ.
func (l *Limiter) Sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweepLocked(l.now())
}
