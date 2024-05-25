package widget

import (
	"errors"
	"fmt"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
)

type Clock struct {
	widgetBase `yaml:",inline"`
	cachedHTML template.HTML `yaml:"-"`
	HourFormat string        `yaml:"hour-format"`
	Timezones  []struct {
		Timezone string `yaml:"timezone"`
		Label    string `yaml:"label"`
	} `yaml:"timezones"`
}

func (widget *Clock) Initialize() error {
	widget.withTitle("Clock").withError(nil)

	if widget.HourFormat == "" {
		widget.HourFormat = "24h"
	} else if widget.HourFormat != "12h" && widget.HourFormat != "24h" {
		return errors.New("invalid hour format for clock widget, must be either 12h or 24h")
	}

	for t := range widget.Timezones {
		if widget.Timezones[t].Timezone == "" {
			return errors.New("missing timezone value for clock widget")
		}

		_, err := time.LoadLocation(widget.Timezones[t].Timezone)

		if err != nil {
			return fmt.Errorf("invalid timezone '%s' for clock widget: %v", widget.Timezones[t].Timezone, err)
		}
	}

	widget.cachedHTML = widget.render(widget, assets.ClockTemplate)

	return nil
}

func (widget *Clock) Render() template.HTML {
	return widget.cachedHTML
}
