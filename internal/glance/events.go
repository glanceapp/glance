package glance

import (
	"sync"
	"time"
)

type event struct {
	Type     string    `json:"type"`
	WidgetID uint64    `json:"widgetId"`
	Time     time.Time `json:"time"`
}

const eventHubChannelBuffer = 8

type eventHub struct {
	mu      sync.Mutex
	clients map[chan event]struct{}
}

func newEventHub() *eventHub {
	return &eventHub{
		clients: make(map[chan event]struct{}),
	}
}

func (h *eventHub) register() chan event {
	ch := make(chan event, eventHubChannelBuffer)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *eventHub) unregister(ch chan event) {
	h.mu.Lock()
	if _, exists := h.clients[ch]; exists {
		delete(h.clients, ch)
		close(ch)
	}
	h.mu.Unlock()
}

// publish broadcasts e to all registered clients. Slow clients are dropped.
func (h *eventHub) publish(e event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- e:
		default:
		}
	}
}

// close shuts down all client channels; called when the application is being replaced.
func (h *eventHub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		close(ch)
	}
	h.clients = make(map[chan event]struct{})
}

func (h *eventHub) clientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}
