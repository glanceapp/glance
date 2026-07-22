package glance

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWidgetContentEndpoint401(t *testing.T) {
	app := newTestApp()
	app.RequiresAuth = true

	req := httptest.NewRequest("GET", "/api/widgets/1/content/", nil)
	w := httptest.NewRecorder()
	app.handleWidgetContentRequest(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestWidgetContentEndpoint404(t *testing.T) {
	app := newTestApp()

	req := httptest.NewRequest("GET", "/api/widgets/9999/content/", nil)
	req.SetPathValue("id", "9999")
	w := httptest.NewRecorder()
	app.handleWidgetContentRequest(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestWidgetContentEndpoint200(t *testing.T) {
	app := newTestApp()

	wgt := &monitorWidget{}
	wgt.setID(widgetIDCounter.Add(1))
	wgt.withError(nil)
	p := &page{}

	app.widgetByID[wgt.GetID()] = wgt
	app.widgetToPage[wgt.GetID()] = p

	req := httptest.NewRequest("GET", "/api/widgets/{id}/content/", nil)
	req.SetPathValue("id", fmt.Sprintf("%d", wgt.GetID()))
	w := httptest.NewRecorder()
	app.handleWidgetContentRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if body := w.Body.String(); body == "" {
		t.Error("expected non-empty HTML body")
	}
}

func TestWidgetContentEndpointNestedWidget(t *testing.T) {
	app := newTestApp()

	child := &monitorWidget{}
	child.setID(widgetIDCounter.Add(1))
	child.withError(nil)

	parent := &groupWidget{}
	parent.setID(widgetIDCounter.Add(1))
	parent.containerWidgetBase.Widgets = widgets{child}

	p := &page{}
	registerWidget(app.widgetByID, app.widgetToPage, parent, p)

	req := httptest.NewRequest("GET", "/api/widgets/{id}/content/", nil)
	req.SetPathValue("id", fmt.Sprintf("%d", child.GetID()))
	w := httptest.NewRecorder()
	app.handleWidgetContentRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for nested widget, got %d", w.Code)
	}
}

func TestWidgetContentResponseContainsDataWidgetID(t *testing.T) {
	app := newTestApp()

	wgt := &monitorWidget{}
	wgt.setID(widgetIDCounter.Add(1))
	wgt.withError(nil)
	p := &page{}

	app.widgetByID[wgt.GetID()] = wgt
	app.widgetToPage[wgt.GetID()] = p

	req := httptest.NewRequest("GET", "/api/widgets/{id}/content/", nil)
	req.SetPathValue("id", fmt.Sprintf("%d", wgt.GetID()))
	w := httptest.NewRecorder()
	app.handleWidgetContentRequest(w, req)

	body := w.Body.String()
	expected := fmt.Sprintf(`data-widget-id="%d"`, wgt.GetID())
	if !strings.Contains(body, expected) {
		t.Errorf("response missing %q; body: %s", expected, body)
	}
}
