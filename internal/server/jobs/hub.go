package jobs

import (
	"encoding/json"
	"sync"

	"github.com/voocel/ainovel-cli/internal/server/api"
)

const (
	hubEventRingSize = 300 // số event gần nhất giữ lại cho client mới kết nối
	hubSubBuffer     = 64  // bộ đệm mỗi subscriber; đầy = subscriber quá chậm, bị ngắt
)

// Message là một message SSE đã mã hoá sẵn. Type là một trong api.SSE*.
type Message struct {
	Type string
	Data []byte // JSON đã mã hoá
}

// Hubs quản lý một Hub cho mỗi truyện, trong bộ nhớ.
type Hubs struct {
	mu   sync.Mutex
	hubs map[string]*Hub
}

func NewHubs() *Hubs { return &Hubs{hubs: make(map[string]*Hub)} }

// Get trả về Hub của truyện, tạo mới nếu chưa có.
func (h *Hubs) Get(bookID string) *Hub {
	h.mu.Lock()
	defer h.mu.Unlock()
	b, ok := h.hubs[bookID]
	if !ok {
		b = &Hub{subs: make(map[*Subscription]struct{})}
		h.hubs[bookID] = b
	}
	return b
}

// Remove xoá Hub của truyện và ngắt toàn bộ subscriber.
func (h *Hubs) Remove(bookID string) {
	h.mu.Lock()
	b := h.hubs[bookID]
	delete(h.hubs, bookID)
	h.mu.Unlock()
	if b != nil {
		b.closeAll()
	}
}

// Hub phát realtime cho các client đang xem một truyện. Mọi hàm an toàn khi gọi
// đồng thời; publisher không bao giờ bị chặn bởi subscriber chậm.
type Hub struct {
	mu       sync.Mutex
	snapshot *api.BookSnapshot
	events   []api.Event // theo thứ tự xuất hiện, tối đa hubEventRingSize
	subs     map[*Subscription]struct{}
}

// SetSnapshot lưu snapshot mới nhất và phát cho các subscriber.
func (b *Hub) SetSnapshot(s api.BookSnapshot) {
	cp := s
	b.mu.Lock()
	defer b.mu.Unlock()
	b.snapshot = &cp
	b.broadcastLocked(api.SSESnapshot, s)
}

// Snapshot trả về snapshot mới nhất; false nếu chưa có.
func (b *Hub) Snapshot() (api.BookSnapshot, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.snapshot == nil {
		return api.BookSnapshot{}, false
	}
	return *b.snapshot, true
}

// PublishEvent upsert event theo ID (cập nhật giữ nguyên vị trí cũ, ID mới thì
// nối cuối, quá giới hạn thì bỏ event cũ nhất) rồi phát cho subscriber.
func (b *Hub) PublishEvent(e api.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	found := false
	for i := range b.events {
		if b.events[i].ID == e.ID {
			b.events[i] = e
			found = true
			break
		}
	}
	if !found {
		b.events = append(b.events, e)
		if over := len(b.events) - hubEventRingSize; over > 0 {
			b.events = append([]api.Event(nil), b.events[over:]...)
		}
	}
	b.broadcastLocked(api.SSEEvent, e)
}

// PublishStream chỉ phát, không lưu.
func (b *Hub) PublishStream(d api.StreamDelta) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.broadcastLocked(api.SSEStream, d)
}

// PublishStatus chỉ phát, không lưu.
func (b *Hub) PublishStatus(s api.StatusChange) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.broadcastLocked(api.SSEStatus, s)
}

// Subscribe đăng ký nhận message. Trạng thái ban đầu được chụp nguyên tử cùng lúc
// đăng ký nên không có khoảng hở hay trùng lặp so với kênh C.
func (b *Hub) Subscribe() *Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := &Subscription{
		hub:     b,
		ch:      make(chan Message, hubSubBuffer),
		dropped: make(chan struct{}),
		events:  append([]api.Event(nil), b.events...),
	}
	if b.snapshot != nil {
		cp := *b.snapshot
		s.snapshot = &cp
	}
	b.subs[s] = struct{}{}
	return s
}

// broadcastLocked mã hoá một lần rồi gửi không chặn; subscriber đầy bộ đệm bị ngắt.
// Gọi khi đang giữ b.mu.
func (b *Hub) broadcastLocked(typ string, v any) {
	if len(b.subs) == 0 {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		return // các kiểu api đều mã hoá được; bỏ qua nếu bất thường
	}
	msg := Message{Type: typ, Data: data}
	for s := range b.subs {
		select {
		case s.ch <- msg:
		default:
			b.dropLocked(s)
		}
	}
}

func (b *Hub) dropLocked(s *Subscription) {
	if _, ok := b.subs[s]; !ok {
		return
	}
	delete(b.subs, s)
	close(s.dropped)
}

func (b *Hub) closeAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for s := range b.subs {
		b.dropLocked(s)
	}
}

// Subscription là một người nghe trên Hub.
type Subscription struct {
	hub      *Hub
	ch       chan Message
	dropped  chan struct{}
	snapshot *api.BookSnapshot
	events   []api.Event
}

// Initial trả về snapshot (nil nếu chưa có) và các event đã lưu tại thời điểm Subscribe.
func (s *Subscription) Initial() (*api.BookSnapshot, []api.Event) {
	return s.snapshot, s.events
}

// C là kênh message (đệm 64).
func (s *Subscription) C() <-chan Message { return s.ch }

// Dropped đóng khi subscriber quá chậm hoặc Hub bị gỡ.
func (s *Subscription) Dropped() <-chan struct{} { return s.dropped }

// Close huỷ đăng ký; gọi nhiều lần an toàn.
func (s *Subscription) Close() {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	s.hub.dropLocked(s)
}
