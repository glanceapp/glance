package widget

import (
	"context"
	"fmt"
	"html/template"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type Weather struct {
	widgetBase      `yaml:",inline"`
	Location        string          `yaml:"location"`
	HideLocation    bool            `yaml:"hide-location"`
	TemperatureUnit string          `yaml:"temperature-unit"`
	Place           *feed.PlaceJson `yaml:"-"`
	Weather         *feed.Weather   `yaml:"-"`
	TimeLabels      [12]string      `yaml:"-"`
}

var timeLabels = [12]string{"2am", "4am", "6am", "8am", "10am", "12pm", "2pm", "4pm", "6pm", "8pm", "10pm", "12am"}

func (widget *Weather) Initialize() error {
	widget.withTitle("Weather").withCacheOnTheHour()
	widget.TimeLabels = timeLabels
	if widget.TemperatureUnit == "" {
		widget.TemperatureUnit = "celsius"
	}
	if widget.TemperatureUnit != "celsius" && widget.TemperatureUnit != "fahrenheit" {
		return fmt.Errorf("invalid temperature unit: %s", widget.TemperatureUnit)
	}

	place, err := feed.FetchPlaceFromName(widget.Location)

	if err != nil {
		return fmt.Errorf("failed fetching data for %s: %v", widget.Location, err)
	}

	widget.Place = place

	return nil
}

func (widget *Weather) Update(ctx context.Context) {
	weather, err := feed.FetchWeatherForPlace(widget.Place, widget.TemperatureUnit)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.Weather = weather
}

func (widget *Weather) Render() template.HTML {
	return widget.render(widget, assets.WeatherTemplate)
}
