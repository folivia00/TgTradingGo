package web

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Event struct {
	Type string      `json:"type"` // candle|trade|equity|status
	Data interface{} `json:"data"`
}

type sseHub struct {
	mu      sync.Mutex
	clients map[chan string]struct{}
}

func newHub() *sseHub { return &sseHub{clients: map[chan string]struct{}{}} }

func (h *sseHub) Subscribe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}

	ch := make(chan string, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, ch)
		close(ch)
		h.mu.Unlock()
	}()

	// приветственное событие
	fmt.Fprintf(w, "event: status\n")
	fmt.Fprintf(w, "data: {\"msg\":\"connected\"}\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

func (h *sseHub) Broadcast(jsonLine string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- jsonLine:
		default:
		}
	}
}

// mockTicker запускает фоновую рассылку тестовых событий (для W1).
func (s *Server) mockTicker() {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case tm := <-t.C:
			msg := fmt.Sprintf("{\"type\":\"status\",\"data\":{\"ts\":%d,\"msg\":\"tick\"}}", tm.Unix())
			s.hub.Broadcast(msg)
		}
	}
}
