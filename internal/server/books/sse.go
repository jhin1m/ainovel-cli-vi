package books

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/voocel/ainovel-cli/internal/server/api"
	"github.com/voocel/ainovel-cli/internal/server/httpx"
)

const (
	ssePing          = 20 * time.Second // Cloudflare cắt kết nối im lặng sau ~100s
	sseWriteDeadline = 10 * time.Second
)

// acquireSSE giữ một suất kết nối SSE cho user; false khi vượt giới hạn.
func (s *Service) acquireSSE(userID string) bool {
	s.sseMu.Lock()
	defer s.sseMu.Unlock()
	if s.sseTotal >= s.limits.MaxSSETotal || s.ssePerUser[userID] >= s.limits.MaxSSEPerUser {
		return false
	}
	s.sseTotal++
	s.ssePerUser[userID]++
	return true
}

func (s *Service) releaseSSE(userID string) {
	s.sseMu.Lock()
	defer s.sseMu.Unlock()
	s.sseTotal--
	if s.ssePerUser[userID]--; s.ssePerUser[userID] <= 0 {
		delete(s.ssePerUser, userID)
	}
}

// handleEvents là SSE của một truyện. Mỗi lần (kết nối lại): snapshot → trạng thái
// → toàn bộ ring event → live. Client thay timeline mỗi lần kết nối; không dùng
// Last-Event-ID. Kết nối giữ nguyên khi truyện dừng (có thể được tiếp tục).
func (s *Service) handleEvents(w http.ResponseWriter, r *http.Request) {
	u, b, err := s.load(r)
	if err != nil {
		httpx.WriteErr(w, r, err)
		return
	}
	if !s.acquireSSE(u.ID) {
		httpx.WriteErr(w, r, httpx.TooManyRequests(5*time.Second))
		return
	}
	defer s.releaseSSE(u.ID)

	// Đăng ký trước rồi mới đọc DB: thay đổi xảy ra giữa hai bước vẫn đến qua kênh.
	sub := s.sup.Hubs().Get(b.ID).Subscribe()
	defer sub.Close()
	if b, err = s.repo.Get(r.Context(), u.ID, b.ID); err != nil {
		httpx.WriteErr(w, r, mapNotFound(err))
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("X-Accel-Buffering", "no")
	h.Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)
	write := func(chunk string) error {
		// Deadline cho từng lần ghi: client không đọc thì bị cắt sau ≤10s.
		if err := rc.SetWriteDeadline(time.Now().Add(sseWriteDeadline)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			return err
		}
		if _, err := io.WriteString(w, chunk); err != nil {
			return err
		}
		return rc.Flush()
	}
	send := func(typ string, v any) error {
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return write("event: " + typ + "\ndata: " + string(data) + "\n\n")
	}

	if write("retry: 3000\n\n") != nil {
		return
	}
	snap, events := sub.Initial()
	if snap == nil {
		snap = b.Snapshot
	}
	if snap != nil && send(api.SSESnapshot, snap) != nil {
		return
	}
	if send(api.SSEStatus, api.StatusChange{Status: b.Status, StopReason: b.StopReason, LastError: b.LastError}) != nil {
		return
	}
	for _, e := range events {
		if send(api.SSEEvent, e) != nil {
			return
		}
	}

	ping := time.NewTicker(ssePing)
	defer ping.Stop()
	for {
		select {
		case msg := <-sub.C():
			if write("event: "+msg.Type+"\ndata: "+string(msg.Data)+"\n\n") != nil {
				return
			}
		case <-sub.Dropped():
			// Client đọc chậm: đóng để trình duyệt tự kết nối lại và nhận lại snapshot.
			return
		case <-ping.C:
			if write(": ping\n\n") != nil {
				return
			}
		case <-r.Context().Done():
			return
		case <-s.closing:
			return
		}
	}
}
