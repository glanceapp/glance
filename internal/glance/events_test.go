package glance

import (
	"sync"
	"testing"
	"time"
)

func TestEventHubBroadcast(t *testing.T) {
	h := newEventHub()
	ch1 := h.register()
	ch2 := h.register()

	e := event{Type: "widget-updated", WidgetID: 42, Time: time.Now()}
	h.publish(e)

	select {
	case got := <-ch1:
		if got.WidgetID != e.WidgetID {
			t.Errorf("ch1: got widget id %d, want %d", got.WidgetID, e.WidgetID)
		}
	case <-time.After(time.Second):
		t.Error("ch1: timed out waiting for event")
	}

	select {
	case got := <-ch2:
		if got.WidgetID != e.WidgetID {
			t.Errorf("ch2: got widget id %d, want %d", got.WidgetID, e.WidgetID)
		}
	case <-time.After(time.Second):
		t.Error("ch2: timed out waiting for event")
	}
}

func TestEventHubSlowClientDropped(t *testing.T) {
	h := newEventHub()
	_ = h.register()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < eventHubChannelBuffer+5; i++ {
			h.publish(event{Type: "test", WidgetID: uint64(i)})
		}
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("publish blocked on slow client")
	}
}

func TestEventHubUnregisterClosesChannel(t *testing.T) {
	h := newEventHub()
	ch := h.register()
	h.unregister(ch)

	_, ok := <-ch
	if ok {
		t.Error("channel should be closed after unregister")
	}
}

func TestEventHubCloseAllClients(t *testing.T) {
	h := newEventHub()
	ch1 := h.register()
	ch2 := h.register()

	h.close()

	_, ok1 := <-ch1
	_, ok2 := <-ch2
	if ok1 || ok2 {
		t.Error("all channels should be closed after hub.close()")
	}
	if h.clientCount() != 0 {
		t.Error("client count should be 0 after hub.close()")
	}
}

func TestEventHubConcurrent(t *testing.T) {
	h := newEventHub()
	const numClients = 10
	const numEvents = 50

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		ch := h.register()
		wg.Add(1)
		go func(ch chan event) {
			defer wg.Done()
			for range ch {
			}
		}(ch)
	}

	var publishWg sync.WaitGroup
	for i := 0; i < numEvents; i++ {
		publishWg.Add(1)
		go func(i int) {
			defer publishWg.Done()
			h.publish(event{Type: "test", WidgetID: uint64(i)})
		}(i)
	}
	publishWg.Wait()

	h.close()
	wg.Wait()
}
