package glance

import (
	"errors"
	"fmt"
	"html/template"
	"time"
)

var clockWidgetTemplate = mustParseTemplate("clock.html", "widget-base.html")

type clockWidget struct {
	widgetBase `yaml:",inline"`
	cachedHTML template.HTML `yaml:"-"`
	HourFormat string        `yaml:"hour-format"`
	// Timezone optionally sets the primary clock (date + large time) to a fixed
	// zone instead of the viewer's local timezone. Secondary rows still use timezones:.
	Timezone  string `yaml:"timezone"`
	Timezones []struct {
		Timezone string `yaml:"timezone"`
		Label    string `yaml:"label"`
	} `yaml:"timezones"`
}

func (widget *clockWidget) initialize() error {
	widget.withTitle("Clock").withError(nil)

	if widget.HourFormat == "" {
		widget.HourFormat = "24h"
	} else if widget.HourFormat != "12h" && widget.HourFormat != "24h" {
		return errors.New("hour-format must be either 12h or 24h")
	}

	if widget.Timezone != "" {
		if _, err := time.LoadLocation(widget.Timezone); err != nil {
			return fmt.Errorf("invalid timezone '%s': %v", widget.Timezone, err)
		}
	}

	for t := range widget.Timezones {
		if widget.Timezones[t].Timezone == "" {
			return errors.New("missing timezone value")
		}

		if _, err := time.LoadLocation(widget.Timezones[t].Timezone); err != nil {
			return fmt.Errorf("invalid timezone '%s': %v", widget.Timezones[t].Timezone, err)
		}
	}

	widget.cachedHTML = widget.renderTemplate(widget, clockWidgetTemplate)

	return nil
}

func (widget *clockWidget) Render() template.HTML {
	return widget.cachedHTML
}
