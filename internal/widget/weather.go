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
	HourFormat   string          `yaml:"hour-format"`
	Units        string          `yaml:"units"`
	Place        *feed.PlaceJson `yaml:"-"`
	Weather      *feed.Weather   `yaml:"-"`
	TimeLabels   [12]string      `yaml:"-"`
}

var timeLabels12h = [12]string{"2am", "4am", "6am", "8am", "10am", "12pm", "2pm", "4pm", "6pm", "8pm", "10pm", "12am"}
var timeLabels24h = [12]string{"02:00", "04:00", "06:00", "08:00", "10:00", "12:00", "14:00", "16:00", "18:00", "20:00", "22:00", "00:00"}

func (widget *Weather) Initialize() error {
	widget.withTitle("Weather").withCacheOnTheHour()

	if widget.Location == "" {
		return fmt.Errorf("location must be specified for weather widget")
	}

	if widget.HourFormat == "" || widget.HourFormat == "12h" {
		widget.TimeLabels = timeLabels12h
	} else if widget.HourFormat == "24h" {
		widget.TimeLabels = timeLabels24h
	} else {
		return fmt.Errorf("invalid hour format '%s' for weather widget, must be either 12h or 24h", widget.HourFormat)
	}

	if widget.Units == "" {
		widget.Units = "metric"
	} else if widget.Units != "metric" && widget.Units != "imperial" {
		return fmt.Errorf("invalid units '%s' for weather, must be either metric or imperial", widget.Units)
	}

	return nil
}

func (widget *Weather) Update(ctx context.Context) {
	if widget.Place == nil {
		place, err := feed.FetchPlaceFromName(widget.Location)

		if err != nil {
			widget.withError(err).scheduleEarlyUpdate()
			return
		}

		widget.Place = place
	}

	weather, err := feed.FetchWeatherForPlace(widget.Place, widget.Units)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.Weather = weather
}

func (widget *Weather) Render() template.HTML {
	return widget.render(widget, assets.WeatherTemplate)
}
