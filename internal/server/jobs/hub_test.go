package jobs

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/voocel/ainovel-cli/internal/server/api"
)

func hubRecv(t *testing.T, s *Subscription) Message {
	t.Helper()
	select {
	case m := <-s.C():
		return m
	case <-time.After(2 * time.Second):
		t.Fatal("hết thời gian chờ message")
		return Message{}
	}
}

func TestHubEventUpsertKeepsOrder(t *testing.T) {
	h := NewHubs().Get("b")
	h.PublishEvent(api.Event{ID: "a", Summary: "1"})
	h.PublishEvent(api.Event{ID: "b", Summary: "2"})
	h.PublishEvent(api.Event{ID: "a", Summary: "1x", Finished: true})
	_, evs := h.Subscribe().Initial()
	if len(evs) != 2 || evs[0].ID != "a" || evs[0].Summary != "1x" || evs[1].ID != "b" {
		t.Fatalf("thứ tự/upsert sai: %+v", evs)
	}
}

func TestHubEventRingCapped(t *testing.T) {
	h := NewHubs().Get("b")
	for i := 0; i < 450; i++ {
		h.PublishEvent(api.Event{ID: fmt.Sprint(i)})
	}
	_, evs := h.Subscribe().Initial()
	if len(evs) != 300 || evs[0].ID != "150" || evs[299].ID != "449" {
		t.Fatalf("ring sai: len=%d first=%s", len(evs), evs[0].ID)
	}
}

func TestHubManySubscribersReceive(t *testing.T) {
	h := NewHubs().Get("b")
	subs := make([]*Subscription, 20)
	for i := range subs {
		subs[i] = h.Subscribe()
	}
	h.PublishStream(api.StreamDelta{Delta: "xin"})
	h.PublishStatus(api.StatusChange{Status: api.StatusRunning})
	for _, s := range subs {
		m := hubRecv(t, s)
		if m.Type != api.SSEStream {
			t.Fatalf("type = %s", m.Type)
		}
		var d api.StreamDelta
		if err := json.Unmarshal(m.Data, &d); err != nil || d.Delta != "xin" {
			t.Fatalf("data sai: %s %v", m.Data, err)
		}
		if m := hubRecv(t, s); m.Type != api.SSEStatus {
			t.Fatalf("type = %s", m.Type)
		}
	}
}

func TestHubSlowSubscriberDropped(t *testing.T) {
	h := NewHubs().Get("b")
	slow := h.Subscribe()
	fast := h.Subscribe()
	var wg sync.WaitGroup
	wg.Add(1)
	got := 0
	go func() {
		defer wg.Done()
		for {
			select {
			case <-fast.C():
				got++
			case <-fast.Dropped():
				return
			case <-time.After(300 * time.Millisecond):
				return
			}
		}
	}()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			h.PublishStream(api.StreamDelta{Delta: "x"})
			// Nhường CPU để subscriber nhanh kịp đọc (GOMAXPROCS=1 nếu không nhường thì nó
			// bị đầy bộ đệm và ngắt như subscriber chậm). Subscriber chậm không đọc nên vẫn bị ngắt.
			if i%16 == 15 {
				time.Sleep(time.Millisecond)
			}
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("publisher bị chặn")
	}
	select {
	case <-slow.Dropped():
	default:
		t.Fatal("subscriber chậm chưa bị ngắt")
	}
	wg.Wait()
	if got == 0 {
		t.Fatal("subscriber nhanh không nhận được gì")
	}
}

func TestHubSubscribeMidStream(t *testing.T) {
	h := NewHubs().Get("b")
	h.SetSnapshot(api.BookSnapshot{BookTitle: "T1"})
	h.PublishEvent(api.Event{ID: "e1"})
	s := h.Subscribe()
	h.PublishEvent(api.Event{ID: "e2"})
	snap, evs := s.Initial()
	if snap == nil || snap.BookTitle != "T1" || len(evs) != 1 || evs[0].ID != "e1" {
		t.Fatalf("initial sai: %+v %+v", snap, evs)
	}
	m := hubRecv(t, s)
	if m.Type != api.SSEEvent {
		t.Fatalf("type = %s", m.Type)
	}
	var e api.Event
	_ = json.Unmarshal(m.Data, &e)
	if e.ID != "e2" {
		t.Fatalf("nhận event %s, muốn e2", e.ID)
	}
	if got, ok := h.Snapshot(); !ok || got.BookTitle != "T1" {
		t.Fatal("Snapshot() sai")
	}
}

func TestHubRemoveClosesSubscribers(t *testing.T) {
	hubs := NewHubs()
	s := hubs.Get("b").Subscribe()
	hubs.Remove("b")
	select {
	case <-s.Dropped():
	default:
		t.Fatal("Dropped chưa đóng sau Remove")
	}
	s.Close() // gọi lại không panic
	if hubs.Get("b") == hubs.Get("c") {
		t.Fatal("hub khác truyện phải khác nhau")
	}
}

func TestHubConcurrent(t *testing.T) {
	h := NewHubs().Get("b")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				h.PublishEvent(api.Event{ID: fmt.Sprint(j % 50)})
				h.SetSnapshot(api.BookSnapshot{CompletedCount: j})
			}
		}(i)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				s := h.Subscribe()
				s.Initial()
				s.Close()
			}
		}()
	}
	wg.Wait()
}
