package events

import (
	"encoding/json"
	"sync"
)

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type Hub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: map[chan []byte]struct{}{}}
}

func (h *Hub) Subscribe() chan []byte {
	ch := make(chan []byte, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

func (h *Hub) Publish(eventType string, data any) {
	payload, err := json.Marshal(Event{Type: eventType, Data: data})
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- payload:
		default:
			// Drop if slow consumer
		}
	}
}
