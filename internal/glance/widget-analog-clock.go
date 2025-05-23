package glance

import (
	"errors"
	"fmt"
	"html/template"
	"time"
)

var analogClockWidgetTemplate = mustParseTemplate("analog-clock.html", "widget-base.html")

type analogClockWidget struct {
	widgetBase `yaml:",inline"`
	cachedHTML template.HTML `yaml:"-"`
	HideAmPmIndicator bool   `yaml:"hide-am-pm-indicator"`
	DialMarkers string `yaml:"dial-markers"`
	Timezones  []struct {
		Timezone string `yaml:"timezone"`
		Label    string `yaml:"label"`
	} `yaml:"timezones"`
}

func (widget *analogClockWidget) initialize() error {
	widget.withTitle("AnalogClock").withError(nil)

	for t := range widget.Timezones {
		if widget.Timezones[t].Timezone == "" {
			return errors.New("missing timezone value")
		}

		if _, err := time.LoadLocation(widget.Timezones[t].Timezone); err != nil {
			return fmt.Errorf("invalid timezone '%s': %v", widget.Timezones[t].Timezone, err)
		}
	}

	widget.cachedHTML = widget.renderTemplate(widget, analogClockWidgetTemplate)

	return nil
}

func (widget *analogClockWidget) Render() template.HTML {
	return widget.cachedHTML
}
