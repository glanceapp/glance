package glance

import (
	"testing"
)

func TestRegisterNestedWidgets(t *testing.T) {
	child := &monitorWidget{}
	child.setID(widgetIDCounter.Add(1))

	parent := &groupWidget{}
	parent.setID(widgetIDCounter.Add(1))
	parent.containerWidgetBase.Widgets = widgets{child}

	widgetByID := make(map[uint64]widget)
	widgetToPage := make(map[uint64]*page)
	p := &page{}

	registerWidget(widgetByID, widgetToPage, parent, p)

	if _, ok := widgetByID[parent.GetID()]; !ok {
		t.Error("parent widget not in widgetByID")
	}
	if _, ok := widgetByID[child.GetID()]; !ok {
		t.Error("nested child widget not in widgetByID")
	}
	if widgetToPage[child.GetID()] != p {
		t.Error("child widget not associated with correct page")
	}
}

func TestRegisterNestedWidgetsSplitColumn(t *testing.T) {
	child := &monitorWidget{}
	child.setID(widgetIDCounter.Add(1))

	parent := &splitColumnWidget{}
	parent.setID(widgetIDCounter.Add(1))
	parent.containerWidgetBase.Widgets = widgets{child}

	widgetByID := make(map[uint64]widget)
	widgetToPage := make(map[uint64]*page)
	p := &page{}

	registerWidget(widgetByID, widgetToPage, parent, p)

	if _, ok := widgetByID[child.GetID()]; !ok {
		t.Error("nested child widget in split-column not in widgetByID")
	}
}
