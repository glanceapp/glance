package widget

import (
	"context"
	"fmt"
	"html/template"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type Weather struct {
	widgetBase   `yaml:",inline"`
	Location     string          `yaml:"location"`
	ShowAreaName bool            `yaml:"show-area-name"`
	HideLocation bool            `yaml:"hide-location"`
	Units        string          `yaml:"units"`
	Place        *feed.PlaceJson `yaml:"-"`
	Weather      *feed.Weather   `yaml:"-"`
	TimeLabels   [12]string      `yaml:"-"`
}

var timeLabels = [12]string{"2am", "4am", "6am", "8am", "10am", "12pm", "2pm", "4pm", "6pm", "8pm", "10pm", "12am"}

func (widget *Weather) Initialize() error {
	widget.withTitle("Weather").withCacheOnTheHour()
	widget.TimeLabels = timeLabels

	if widget.Units == "" {
		widget.Units = "metric"
	} else if widget.Units != "metric" && widget.Units != "imperial" {
		return fmt.Errorf("invalid units '%s' for weather, must be either metric or imperial", widget.Units)
	}

	place, err := feed.FetchPlaceFromName(widget.Location)

	if err != nil {
		return fmt.Errorf("failed fetching data for %s: %v", widget.Location, err)
	}

	widget.Place = place

	return nil
}

func (widget *Weather) Update(ctx context.Context) {
	weather, err := feed.FetchWeatherForPlace(widget.Place, widget.Units)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.Weather = weather
}

func (widget *Weather) Render() template.HTML {
	return widget.render(widget, assets.WeatherTemplate)
}
