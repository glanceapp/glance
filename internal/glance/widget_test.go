package glance

import (
	"strings"
	"testing"
	"time"
)

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
