package glance

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestApp() *application {
	return &application{
		slugToPage:   make(map[string]*page),
		widgetByID:   make(map[uint64]widget),
		widgetToPage: make(map[uint64]*page),
		hub:          newEventHub(),
	}
}

func TestSSEEndpointUnauthorized(t *testing.T) {
	app := newTestApp()
	app.RequiresAuth = true

	req := httptest.NewRequest("GET", "/api/events", nil)
	w := httptest.NewRecorder()
	app.handleSSERequest(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestSSEEndpointContentType(t *testing.T) {
	app := newTestApp()

	srv := httptest.NewServer(http.HandlerFunc(app.handleSSERequest))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/events")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}
}

func TestSSEEndpointReceivesEvent(t *testing.T) {
	app := newTestApp()

	srv := httptest.NewServer(http.HandlerFunc(app.handleSSERequest))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/events")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Give the handler time to register with the hub
	time.Sleep(50 * time.Millisecond)

	app.hub.publish(event{Type: "widget-updated", WidgetID: 99})

	eventCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event:") {
				eventCh <- line
				return
			}
		}
	}()

	select {
	case line := <-eventCh:
		if !strings.Contains(line, "widget-updated") {
			t.Errorf("unexpected event line: %s", line)
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for SSE event")
	}
}

func TestSSEEndpointDisconnectCleansUp(t *testing.T) {
	app := newTestApp()

	srv := httptest.NewServer(http.HandlerFunc(app.handleSSERequest))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/events")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	// Give the handler time to register
	time.Sleep(50 * time.Millisecond)

	if app.hub.clientCount() != 1 {
		t.Errorf("expected 1 client, got %d", app.hub.clientCount())
	}

	resp.Body.Close()

	// Give the handler time to unregister
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if app.hub.clientCount() == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Errorf("client not unregistered after disconnect; count: %d", app.hub.clientCount())
}
