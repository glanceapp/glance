package glance

import (
	"strings"
	"testing"
	"time"
)

func TestRegisterWidgetIncludesContainerChildren(t *testing.T) {
	leaf1 := &clockWidget{}
	leaf1.setID(101)
	leaf2 := &clockWidget{}
	leaf2.setID(102)
	leaf3 := &clockWidget{}
	leaf3.setID(103)

	group := &groupWidget{}
	group.setID(200)
	group.containerWidgetBase.Widgets = widgets{leaf1, leaf2}

	split := &splitColumnWidget{}
	split.setID(300)
	split.containerWidgetBase.Widgets = widgets{group, leaf3}

	app := &application{widgetByID: make(map[uint64]widget)}
	app.registerWidget(split)

	for _, id := range []uint64{101, 102, 103, 200, 300} {
		if _, ok := app.widgetByID[id]; !ok {
			t.Errorf("widget id %d was not registered", id)
		}
	}
}

func TestWidgetValidateRefreshInterval(t *testing.T) {
	tests := []struct {
		name        string
		widgetType  string
		interval    time.Duration
		errContains string
	}{
		{name: "no interval is allowed on any type", widgetType: "clock", interval: 0},
		{name: "valid interval on rss", widgetType: "rss", interval: 5 * time.Second},
		{name: "valid interval on hacker-news", widgetType: "hacker-news", interval: 1 * time.Minute},
		{name: "below minimum", widgetType: "rss", interval: 4 * time.Second, errContains: "at least 5s"},
		{name: "disallowed: clock", widgetType: "clock", interval: 1 * time.Minute, errContains: `type "clock"`},
		{name: "disallowed: calendar", widgetType: "calendar", interval: 1 * time.Minute, errContains: `type "calendar"`},
		{name: "disallowed: to-do", widgetType: "to-do", interval: 1 * time.Minute, errContains: `type "to-do"`},
		{name: "disallowed: iframe", widgetType: "iframe", interval: 1 * time.Minute, errContains: `type "iframe"`},
		{name: "disallowed: html", widgetType: "html", interval: 1 * time.Minute, errContains: `type "html"`},
		{name: "disallowed: group", widgetType: "group", interval: 1 * time.Minute, errContains: `type "group"`},
		{name: "disallowed: split-column", widgetType: "split-column", interval: 1 * time.Minute, errContains: `type "split-column"`},
		{name: "disallowed: search", widgetType: "search", interval: 1 * time.Minute, errContains: `type "search"`},
		{name: "disallowed: bookmarks", widgetType: "bookmarks", interval: 1 * time.Minute, errContains: `type "bookmarks"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := &widgetBase{Type: tc.widgetType, RefreshInterval: durationField(tc.interval)}
			err := w.validate()
			if tc.errContains == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errContains)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("expected error containing %q, got: %v", tc.errContains, err)
			}
		})
	}
}
