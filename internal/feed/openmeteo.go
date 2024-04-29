package feed

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"time"

	_ "time/tzdata"
)

type PlacesResponseJson struct {
	Results []PlaceJson
}

type PlaceJson struct {
	Name      string
	Latitude  float64
	Longitude float64
	Timezone  string
	Country   string
	location  *time.Location
}

type WeatherResponseJson struct {
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
	CurrentUnits struct {
		ApparentTemperature string `json:"apparent_temperature"`
	} `json:"current_units"`
}

type weatherColumn struct {
	Temperature      int
	Scale            float64
	HasPrecipitation bool
}

func FetchPlaceFromName(location string) (*PlaceJson, error) {
	requestUrl := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=en&format=json", url.QueryEscape(location))
	request, _ := http.NewRequest("GET", requestUrl, nil)
	responseJson, err := decodeJsonFromRequest[PlacesResponseJson](defaultClient, request)

	if err != nil {
		return nil, fmt.Errorf("could not fetch places data: %v", err)
	}

	if len(responseJson.Results) == 0 {
		return nil, fmt.Errorf("no places found for %s", location)
	}

	place := &responseJson.Results[0]

	loc, err := time.LoadLocation(place.Timezone)

	if err != nil {
		return nil, fmt.Errorf("could not load location: %v", err)
	}

	place.location = loc

	return place, nil
}

func barIndexFromHour(h int) int {
	return h / 2
}

// TODO: bunch of spaget, refactor
// TODO: allow changing between C and F
func FetchWeatherForPlace(place *PlaceJson, temperatureUnit string) (*Weather, error) {
	query := url.Values{}

	query.Add("latitude", fmt.Sprintf("%f", place.Latitude))
	query.Add("longitude", fmt.Sprintf("%f", place.Longitude))
	query.Add("timeformat", "unixtime")
	query.Add("timezone", place.Timezone)
	query.Add("forecast_days", "1")
	query.Add("current", "temperature_2m,apparent_temperature,weather_code,wind_speed_10m")
	query.Add("hourly", "temperature_2m,precipitation_probability")
	query.Add("daily", "sunrise,sunset")
	query.Add("temperature_unit", temperatureUnit)

	requestUrl := "https://api.open-meteo.com/v1/forecast?" + query.Encode()
	request, _ := http.NewRequest("GET", requestUrl, nil)
	responseJson, err := decodeJsonFromRequest[WeatherResponseJson](defaultClient, request)

	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoContent, err)
	}

	now := time.Now().In(place.location)
	bars := make([]weatherColumn, 0, 24)
	currentBar := barIndexFromHour(now.Hour())
	sunriseBar := barIndexFromHour(time.Unix(int64(responseJson.Daily.Sunrise[0]), 0).In(place.location).Hour())
	sunsetBar := barIndexFromHour(time.Unix(int64(responseJson.Daily.Sunset[0]), 0).In(place.location).Hour()) - 1

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

		for i := 0; i < 12; i++ {
			bars = append(bars, weatherColumn{
				Temperature:      temperatures[i],
				Scale:            float64(temperatures[i]-minT) / float64(maxT-minT),
				HasPrecipitation: precipitations[i],
			})
		}
	}

	return &Weather{
		Temperature:             int(responseJson.Current.Temperature),
		ApparentTemperature:     int(responseJson.Current.ApparentTemperature),
		ApparentTemperatureUnit: responseJson.CurrentUnits.ApparentTemperature,
		WeatherCode:             responseJson.Current.WeatherCode,
		CurrentColumn:           currentBar,
		SunriseColumn:           sunriseBar,
		SunsetColumn:            sunsetBar,
		Columns:                 bars,
	}, nil
}
