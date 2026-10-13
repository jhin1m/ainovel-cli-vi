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
	entries map[string]*limiterEntry
	now     func() time.Time
}

type limiterEntry struct {
	lim  *rate.Limiter
	seen time.Time
}

// NewLimiter cho phép perMinute lần mỗi phút (burst = perMinute) cho mỗi khoá.
func NewLimiter(perMinute int) *Limiter {
	return &Limiter{
		every:   rate.Limit(float64(perMinute) / 60),
		burst:   perMinute,
		entries: make(map[string]*limiterEntry),
		now:     time.Now,
	}
}

// maxLimiterEntries: trần số khoá theo dõi. Vượt trần (bị dội khoá giả) thì từ
// chối khoá mới — đóng an toàn thay vì để bộ nhớ phình vô hạn.
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
		if len(l.entries) >= maxLimiterEntries {
			l.sweepLocked(now)
			if len(l.entries) >= maxLimiterEntries {
				return false, time.Minute
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

// sweepLocked bỏ khoá không dùng quá 10 phút (đã hồi đầy lượt).
func (l *Limiter) sweepLocked(now time.Time) {
	for k, e := range l.entries {
		if now.Sub(e.seen) > 10*time.Minute {
			delete(l.entries, k)
		}
	}
}

// Sweep dọn định kỳ.
func (l *Limiter) Sweep() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweepLocked(l.now())
}
