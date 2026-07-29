package glance

import (
	"strings"
	"testing"
	"time"
)

func TestMonitorWidgetHideDetails(t *testing.T) {
	for _, style := range []string{"", "compact"} {
		t.Run(style, func(t *testing.T) {
			widget := &monitorWidget{
				Style:       style,
				HideDetails: true,
			}
			widget.Sites = make([]struct {
				*SiteStatusRequest `yaml:",inline"`
				Status             *siteStatus     `yaml:"-"`
				URL                string          `yaml:"-"`
				ErrorURL           string          `yaml:"error-url"`
				Title              string          `yaml:"title"`
				Icon               customIconField `yaml:"icon"`
				SameTab            bool            `yaml:"same-tab"`
				StatusText         string          `yaml:"-"`
				StatusStyle        string          `yaml:"-"`
				HideDetails        bool            `yaml:"-"`
				AltStatusCodes     []int           `yaml:"alt-status-codes"`
			}, 1)
			widget.Sites[0].Title = "Service"
			widget.Sites[0].Status = &siteStatus{Code: 200, ResponseTime: 123 * time.Millisecond}
			widget.Sites[0].StatusText = "OK"
			widget.Sites[0].StatusStyle = "ok"

			if err := widget.initialize(); err != nil {
				t.Fatalf("initialize monitor widget: %v", err)
			}

			rendered := string(widget.Render())
			if strings.Contains(rendered, "123ms") || strings.Contains(rendered, ">OK<") {
				t.Fatal("expected status details to be hidden")
			}
		})
	}
}
