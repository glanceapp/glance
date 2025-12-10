package glance

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	_ "time/tzdata"
)

var weatherWidgetTemplate = mustParseTemplate("weather.html", "widget-base.html")

type weatherWidget struct {
	widgetBase   `yaml:",inline"`
	Location     string                      `yaml:"location"`
	ShowAreaName bool                        `yaml:"show-area-name"`
	HideLocation bool                        `yaml:"hide-location"`
	HourFormat   string                      `yaml:"hour-format"`
	Units        string                      `yaml:"units"`
	Place        *openMeteoPlaceResponseJson `yaml:"-"`
	Weather      *weather                    `yaml:"-"`
	TimeLabels   [12]string                  `yaml:"-"`
}

var timeLabels12h = [12]string{"2am", "4am", "6am", "8am", "10am", "12pm", "2pm", "4pm", "6pm", "8pm", "10pm", "12am"}
var timeLabels24h = [12]string{"02:00", "04:00", "06:00", "08:00", "10:00", "12:00", "14:00", "16:00", "18:00", "20:00", "22:00", "00:00"}

func (widget *weatherWidget) initialize() error {
	widget.withTitle("Weather").withCacheOnTheHour()

	if widget.Location == "" {
		return fmt.Errorf("location is required")
	}

	if widget.HourFormat == "" || widget.HourFormat == "12h" {
		widget.TimeLabels = timeLabels12h
	} else if widget.HourFormat == "24h" {
		widget.TimeLabels = timeLabels24h
	} else {
		return errors.New("hour-format must be either 12h or 24h")
	}

	if widget.Units == "" {
		widget.Units = "metric"
	} else if widget.Units != "metric" && widget.Units != "imperial" {
		return errors.New("units must be either metric or imperial")
	}

	return nil
}

func (widget *weatherWidget) update(ctx context.Context) {
	if widget.Place == nil {
		place, err := fetchOpenMeteoPlaceFromName(widget.Location)
		if err != nil {
			widget.withError(err).scheduleEarlyUpdate()
			return
		}

		widget.Place = place
	}

	weather, err := fetchWeatherForOpenMeteoPlace(widget.Place, widget.Units)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.Weather = weather
}

func (widget *weatherWidget) Render() template.HTML {
	return widget.renderTemplate(widget, weatherWidgetTemplate)
}

type weather struct {
	Temperature         int
	ApparentTemperature int
	WeatherCode         int
	CurrentColumn       int
	SunriseColumn       int
	SunsetColumn        int
	Columns             []weatherColumn
}

func (w *weather) WeatherCodeAsString() string {
	if weatherCode, ok := weatherCodeTable[w.WeatherCode]; ok {
		return weatherCode
	}

	return ""
}

type openMeteoPlacesResponseJson struct {
	Results []openMeteoPlaceResponseJson
}

type openMeteoPlaceResponseJson struct {
	Name      string
	Area      string `json:"admin1"`
	Latitude  float64
	Longitude float64
	Timezone  string
	Country   string
	location  *time.Location
}

type openMeteoWeatherResponseJson struct {
	Daily struct {
		Sunrise []int64 `json:"sunrise"`
		Sunset  []int64 `json:"sunset"`
	} `json:"daily"`

	Hourly struct {
		Temperature              []float64 `json:"temperature_2m"`
		PrecipitationProbability []int     `json:"precipitation_probability"`
	} `json:"hourly"`

	Current struct {
		Temperature         float64 `json:"temperature_2m"`
		ApparentTemperature float64 `json:"apparent_temperature"`
		WeatherCode         int     `json:"weather_code"`
	} `json:"current"`
}

type weatherColumn struct {
	Temperature      int
	Scale            float64
	HasPrecipitation bool
}

var commonCountryAbbreviations = map[string]string{
	"US":  "United States",
	"USA": "United States",
	"UK":  "United Kingdom",
}

func expandCountryAbbreviations(name string) string {
	if expanded, ok := commonCountryAbbreviations[strings.TrimSpace(name)]; ok {
		return expanded
	}

	return name
}

// Separates the location that Open Meteo accepts from the administrative area
// which can then be used to filter to the correct place after the list of places
// has been retrieved. Also expands abbreviations since Open Meteo does not accept
// country names like "US", "USA" and "UK"
func parsePlaceName(name string) (string, string) {
	parts := strings.Split(name, ",")

	if len(parts) == 1 {
		return name, ""
	}

	if len(parts) == 2 {
		return parts[0] + ", " + expandCountryAbbreviations(parts[1]), ""
	}

	return parts[0] + ", " + expandCountryAbbreviations(parts[2]), strings.TrimSpace(parts[1])
}

func fetchOpenMeteoPlaceFromName(location string) (*openMeteoPlaceResponseJson, error) {
	location, area := parsePlaceName(location)
	requestUrl := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=20&language=en&format=json", url.QueryEscape(location))
	request, _ := http.NewRequest("GET", requestUrl, nil)
	responseJson, err := decodeJsonFromRequest[openMeteoPlacesResponseJson](defaultHTTPClient, request)
	if err != nil {
		return nil, fmt.Errorf("fetching places data: %v", err)
	}

	if len(responseJson.Results) == 0 {
		return nil, fmt.Errorf("no places found for %s", location)
	}

	var place *openMeteoPlaceResponseJson

	if area != "" {
		area = strings.ToLower(area)

		for i := range responseJson.Results {
			if strings.ToLower(responseJson.Results[i].Area) == area {
				place = &responseJson.Results[i]
				break
			}
		}

		if place == nil {
			return nil, fmt.Errorf("no place found for %s in %s", location, area)
		}
	} else {
		place = &responseJson.Results[0]
	}

	loc, err := time.LoadLocation(place.Timezone)
	if err != nil {
		return nil, fmt.Errorf("loading location: %v", err)
	}

	place.location = loc

	return place, nil
}

func fetchWeatherForOpenMeteoPlace(place *openMeteoPlaceResponseJson, units string) (*weather, error) {
	query := url.Values{}
	var temperatureUnit string

	if units == "imperial" {
		temperatureUnit = "fahrenheit"
	} else {
		temperatureUnit = "celsius"
	}

	query.Add("latitude", fmt.Sprintf("%f", place.Latitude))
	query.Add("longitude", fmt.Sprintf("%f", place.Longitude))
	query.Add("timeformat", "unixtime")
	query.Add("timezone", place.Timezone)
	query.Add("forecast_days", "1")
	query.Add("current", "temperature_2m,apparent_temperature,weather_code")
	query.Add("hourly", "temperature_2m,precipitation_probability")
	query.Add("daily", "sunrise,sunset")
	query.Add("temperature_unit", temperatureUnit)

	requestUrl := "https://api.open-meteo.com/v1/forecast?" + query.Encode()
	request, _ := http.NewRequest("GET", requestUrl, nil)
	responseJson, err := decodeJsonFromRequest[openMeteoWeatherResponseJson](defaultHTTPClient, request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errNoContent, err)
	}

	now := time.Now().In(place.location)
	bars := make([]weatherColumn, 0, 24)
	currentBar := now.Hour() / 2
	sunriseBar := (time.Unix(int64(responseJson.Daily.Sunrise[0]), 0).In(place.location).Hour()) / 2
	sunsetBar := (time.Unix(int64(responseJson.Daily.Sunset[0]), 0).In(place.location).Hour() - 1) / 2

	if sunsetBar < 0 {
		sunsetBar = 0
	}

	if len(responseJson.Hourly.Temperature) == 24 {
		temperatures := make([]int, 12)
		precipitations := make([]bool, 12)

		t := responseJson.Hourly.Temperature
		p := responseJson.Hourly.PrecipitationProbability

		for i := 0; i < 24; i += 2 {
			if i/2 == currentBar {
				temperatures[i/2] = int(responseJson.Current.Temperature)
			} else {
				temperatures[i/2] = int(math.Round((t[i] + t[i+1]) / 2))
			}

			precipitations[i/2] = (p[i]+p[i+1])/2 > 75
		}

		minT := slices.Min(temperatures)
		maxT := slices.Max(temperatures)

		temperaturesRange := float64(maxT - minT)

		for i := 0; i < 12; i++ {
			bars = append(bars, weatherColumn{
				Temperature:      temperatures[i],
				HasPrecipitation: precipitations[i],
			})

			if temperaturesRange > 0 {
				bars[i].Scale = float64(temperatures[i]-minT) / temperaturesRange
			} else {
				bars[i].Scale = 1
			}
		}
	}

	return &weather{
		Temperature:         int(responseJson.Current.Temperature),
		ApparentTemperature: int(responseJson.Current.ApparentTemperature),
		WeatherCode:         responseJson.Current.WeatherCode,
		CurrentColumn:       currentBar,
		SunriseColumn:       sunriseBar,
		SunsetColumn:        sunsetBar,
		Columns:             bars,
	}, nil
}

var weatherCodeTable = map[int]string{
	0:  "Clear Sky",
	1:  "Mainly Clear",
	2:  "Partly Cloudy",
	3:  "Overcast",
	45: "Fog",
	48: "Rime Fog",
	51: "Drizzle",
	53: "Drizzle",
	55: "Drizzle",
	56: "Drizzle",
	57: "Drizzle",
	61: "Rain",
	63: "Moderate Rain",
	65: "Heavy Rain",
	66: "Freezing Rain",
	67: "Freezing Rain",
	71: "Snow",
	73: "Moderate Snow",
	75: "Heavy Snow",
	77: "Snow Grains",
	80: "Rain",
	81: "Moderate Rain",
	82: "Heavy Rain",
	85: "Snow",
	86: "Snow",
	95: "Thunderstorm",
	96: "Thunderstorm",
	99: "Thunderstorm",
}
