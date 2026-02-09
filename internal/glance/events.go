package glance

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type eventHub struct {
	mu                        sync.Mutex
	clients                   map[chan []byte]struct{}
	lastMonitorEventTimes     map[uint64]time.Time // debounce per widget
	monitorEventDebounceTime  time.Duration
	lastEvents                map[string][]byte // recent events cache keyed by eventType|widget
}

func newEventHub() *eventHub {
	return &eventHub{
		clients:                  make(map[chan []byte]struct{}),
		lastMonitorEventTimes:    make(map[uint64]time.Time),
		monitorEventDebounceTime: 5 * time.Second,
		lastEvents:               make(map[string][]byte),
	}
}

func (h *eventHub) register() chan []byte {
	ch := make(chan []byte, 8)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	clientsCount := len(h.clients)
	// snapshot recent events to send after unlocking
	recent := make([][]byte, 0, len(h.lastEvents))
	for _, v := range h.lastEvents {
		// copy to avoid race
		b := make([]byte, len(v))
		copy(b, v)
		recent = append(recent, b)
	}
	h.mu.Unlock()
	log.Printf("SSE client registered (clients=%d)", clientsCount)

	// deliver recent events asynchronously so register() remains non-blocking
	go func() {
		for _, ev := range recent {
			select {
			case ch <- ev:
			default:
				// drop if client channel is full
			}
		}
	}()
	return ch
}

func (h *eventHub) unregister(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	clientsCount := len(h.clients)
	h.mu.Unlock()
	close(ch)
	log.Printf("SSE client unregistered (clients=%d)", clientsCount)
}

func (h *eventHub) broadcast(msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
			// drop message for slow client
		}
	}
}

// global hub instance
var globalEventHub *eventHub

// handleEvents serves an SSE stream to the client
func (a *application) handleEvents(w http.ResponseWriter, r *http.Request) {
	if globalEventHub == nil {
		http.Error(w, "events not available", http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// security: simple auth via same cookie handling as other endpoints
	if a.handleUnauthorizedResponse(w, r, showUnauthorizedJSON) {
		return
	}

	// set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	msgCh := globalEventHub.register()
	defer globalEventHub.unregister(msgCh)

	log.Printf("handleEvents: client connected %s", r.RemoteAddr)

	ctx := r.Context()

	// send a ping every 30s to keep connection alive
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// initial comment to establish stream
	w.Write([]byte(": ok\n\n"))
	flusher.Flush()

	for {
		select {
		case <-ctx.Done():
			log.Printf("handleEvents: client context done %s", r.RemoteAddr)
			return
		case msg, ok := <-msgCh:
			if !ok {
				return
			}
			// write data and check for errors so we can log disconnect causes
			if _, err := w.Write([]byte("data: ")); err != nil {
				log.Printf("handleEvents: write error (data prefix) %s: %v", r.RemoteAddr, err)
				return
			}
			if _, err := w.Write(msg); err != nil {
				log.Printf("handleEvents: write error (msg) %s: %v", r.RemoteAddr, err)
				return
			}
			if _, err := w.Write([]byte("\n\n")); err != nil {
				log.Printf("handleEvents: write error (terminator) %s: %v", r.RemoteAddr, err)
				return
			}
			// Flush and check; Flush has no error return so rely on ctx.Done if connection closed
			flusher.Flush()
		case <-pingTicker.C:
			// send a keep-alive comment
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				log.Printf("handleEvents: write error (ping) %s: %v", r.RemoteAddr, err)
				return
			}
			flusher.Flush()
		}
	}
}

// parseWidgetID extracts uint64 widget_id from various numeric types in a map
func parseWidgetID(m map[string]any) (uint64, bool) {
	if raw, exists := m["widget_id"]; exists {
		switch v := raw.(type) {
		case float64:
			return uint64(v), true
		case int:
			return uint64(v), true
		case int64:
			return uint64(v), true
		case uint64:
			return v, true
		case json.Number:
			if n, err := v.Int64(); err == nil {
				return uint64(n), true
			}
		}
	}
	return 0, false
}

// helper to publish JSON event
func publishEvent(eventType string, payload any) {
	if globalEventHub == nil {
		return
	}

	// debounce monitor events per widget to avoid flapping
	if eventType == "monitor:site_changed" {
		if payloadMap, ok := payload.(map[string]any); ok {
			if widgetIDUint, ok := parseWidgetID(payloadMap); ok {
				globalEventHub.mu.Lock()
				if lastTime, exists := globalEventHub.lastMonitorEventTimes[widgetIDUint]; exists {
					if time.Since(lastTime) < globalEventHub.monitorEventDebounceTime {
						globalEventHub.mu.Unlock()
						return // drop event, too soon
					}
				}
				globalEventHub.lastMonitorEventTimes[widgetIDUint] = time.Now()
				globalEventHub.mu.Unlock()
			}
		}
	}

	wrapper := map[string]any{
		"type": eventType,
		"time": time.Now().Unix(),
		"data": payload,
	}

	b, err := json.Marshal(wrapper)
	if err != nil {
		log.Printf("failed to marshal event: %v", err)
		return
	}

	// cache recent monitor/custom-api/page events so reconnecting clients can catch up
	if globalEventHub != nil {
		key := ""
		if eventType == "monitor:site_changed" || eventType == "custom-api:data_changed" {
			if payloadMap, ok := payload.(map[string]any); ok {
				if widgetIDUint, ok := parseWidgetID(payloadMap); ok {
					key = eventType + "|" + strconv.FormatUint(widgetIDUint, 10)
				}
			}
		} else if eventType == "page:update" {
			if payloadMap, ok := payload.(map[string]any); ok {
				if slug, ok := payloadMap["slug"].(string); ok {
					key = eventType + "|" + slug
				}
			}
		}

		globalEventHub.mu.Lock()
		clientsCount := len(globalEventHub.clients)
		if key != "" {
			// store a copy
			copyB := make([]byte, len(b))
			copy(copyB, b)
			globalEventHub.lastEvents[key] = copyB
		}
		globalEventHub.mu.Unlock()

		log.Printf("publishing event %s to %d clients", eventType, clientsCount)
	}

	globalEventHub.broadcast(b)
}
