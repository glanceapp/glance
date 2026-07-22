package glance

import (
	"context"
	"html/template"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// fakeWidget is a controllable widget for ticker tests.
type fakeWidget struct {
	widgetBase
	updateCount  atomic.Int32
	renderedHTML template.HTML
	onUpdate     func()
}

func (fw *fakeWidget) initialize() error { return nil }
func (fw *fakeWidget) update(_ context.Context) {
	fw.updateCount.Add(1)
	if fw.onUpdate != nil {
		fw.onUpdate()
	}
}
func (fw *fakeWidget) Render() template.HTML {
	return fw.renderedHTML
}
func (fw *fakeWidget) handleRequest(_ http.ResponseWriter, _ *http.Request) {}

func newFakeWidget(nextUpdate time.Time, html template.HTML) *fakeWidget {
	fw := &fakeWidget{renderedHTML: html}
	fw.setID(widgetIDCounter.Add(1))
	fw.cacheType = cacheTypeDuration
	fw.nextUpdate = nextUpdate
	return fw
}

func TestTickerSkipsUpToDateWidgets(t *testing.T) {
	app := newTestApp()
	p := &page{}

	fw := newFakeWidget(time.Now().Add(time.Hour), "<div>fresh</div>")
	p.HeadWidgets = widgets{fw}
	app.slugToPage["test"] = p

	app.tickPage(context.Background(), p)

	if n := fw.updateCount.Load(); n != 0 {
		t.Errorf("expected 0 updates for fresh widget, got %d", n)
	}
}

func TestTickerUpdatesStaleWidget(t *testing.T) {
	app := newTestApp()
	p := &page{}

	fw := newFakeWidget(time.Now().Add(-time.Minute), "<div>stale</div>")
	p.HeadWidgets = widgets{fw}
	app.slugToPage["test"] = p

	app.tickPage(context.Background(), p)

	if n := fw.updateCount.Load(); n != 1 {
		t.Errorf("expected 1 update for stale widget, got %d", n)
	}
}

func TestTickerEmitsEventOnChange(t *testing.T) {
	app := newTestApp()
	p := &page{}

	fw := newFakeWidget(time.Now().Add(-time.Minute), "<div>old</div>")
	fw.onUpdate = func() {
		fw.renderedHTML = "<div>new</div>"
	}
	p.HeadWidgets = widgets{fw}
	app.slugToPage["test"] = p

	ch := app.hub.register()

	app.tickPage(context.Background(), p)

	select {
	case e := <-ch:
		if e.Type != "widget-updated" {
			t.Errorf("expected widget-updated event, got %s", e.Type)
		}
		if e.WidgetID != fw.GetID() {
			t.Errorf("expected widget ID %d, got %d", fw.GetID(), e.WidgetID)
		}
	case <-time.After(time.Second):
		t.Error("expected event not received")
	}
}

func TestTickerNoEventWhenContentUnchanged(t *testing.T) {
	app := newTestApp()
	p := &page{}

	const unchanged = template.HTML("<div>same</div>")
	fw := newFakeWidget(time.Now().Add(-time.Minute), unchanged)
	p.HeadWidgets = widgets{fw}
	app.slugToPage["test"] = p

	ch := app.hub.register()

	app.tickPage(context.Background(), p)

	select {
	case e := <-ch:
		t.Errorf("unexpected event emitted: %+v", e)
	case <-time.After(100 * time.Millisecond):
		// correct: no event
	}
}
