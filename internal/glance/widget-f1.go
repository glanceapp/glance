package glance

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"time"
)

var formula1WidgetTemplate = mustParseTemplate("f1.html", "widget-base.html")

const (
	NextRaceApi                 = "https://f1api.dev/api/current/next"
	LastRaceApi                 = "https://f1api.dev/api/current/last/race"
	DriversChampionshipApi      = "https://f1api.dev/api/current/drivers-championship"
	ConstructorsChampionshipApi = "https://f1api.dev/api/current/constructors-championship"
	DateFormatString            = "Jan 02, 2006 - 03:04 PM"
)

type formula1Widget struct {
	widgetBase   `yaml:",inline"`
	Data         formula1WidgetData `yaml:"-"`
	SessionType  string             `yaml:"session-type"` // supports: race, major, all
	ShowLastRace bool               `yaml:"show-last-race"`
	ShowWdcOrder bool               `yaml:"show-wdc-order"`
	ShowWccOrder bool               `yaml:"show-wcc-order"`
}

func (widget *formula1Widget) initialize() error {
	widget.withTitle("Formula 1").withCacheDuration(1 * time.Hour)

	return nil
}

func (widget *formula1Widget) update(ctx context.Context) {
	data, err := getWidgetData(widget)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.Data = data
}

func (widget *formula1Widget) Render() template.HTML {
	return widget.renderTemplate(widget, formula1WidgetTemplate)
}

type Datetime struct {
	Date string
	Time string
}

type DatetimeWithTitle struct {
	Datetime Datetime
	Title    string
}

type GrandPrix struct {
	RaceName string
	Round    int
	Url      string
	Schedule struct {
		Race        Datetime
		Qualy       Datetime
		Fp1         Datetime
		Fp2         Datetime
		Fp3         Datetime
		SprintQualy Datetime
		SprintRace  Datetime
	}
	Circuit struct {
		CircuitName string
		Country     string
		City        string
	}
}

type formula1NextRaceHttpResponse struct {
	Race [1]GrandPrix
}

type formula1LastRaceHttpResponse struct {
	Races struct {
		RaceName string
		Round    int
		Date     string
		Time     string
		Results  []Result
	}
}

type formula1WdcHttpResponse struct {
	Drivers_Championship []Classification
}

type formula1WccHttpResponse struct {
	Constructors_Championship []Classification
}

type Session struct {
	Title       string
	Round       int
	SessionName string
	Unix        int64
	DateString  string
	Circuit     string
	Country     string
	City        string
}

type Driver struct {
	Number    int
	ShortName string
	Name      string
	Surname   string
}

type Team struct {
	TeamName string
	Country  string
}

type Result struct {
	Position any
	Points   int
	Time     string
	Driver   Driver
	Team     Team
}

type Classification struct {
	Points   int
	Position int
	Team     Team
	Driver   Driver
}

type LastRace struct {
	RaceName   string
	Round      int
	DateString string
	Url        string
	Results    []Result
}

type formula1WidgetData struct {
	Session  Session
	LastRace LastRace
	WdcOrder []Classification
	WccOrder []Classification
}

func getWidgetData(w *formula1Widget) (formula1WidgetData, error) {
	session, err := getNextSession(w.SessionType)

	if err != nil {
		return formula1WidgetData{}, err
	}

	data := formula1WidgetData{
		Session: session,
	}

	if w.ShowLastRace {
		lastRace, err := getLastRaceResults()
		if err != nil {
			return formula1WidgetData{}, err
		}
		data.LastRace = lastRace
	}

	if w.ShowWdcOrder {
		wdcOrder, err := getWdcOrder()
		if err != nil {
			return formula1WidgetData{}, err
		}
		data.WdcOrder = wdcOrder
	}

	if w.ShowWccOrder {
		wccOrder, err := getWccOrder()
		if err != nil {
			return formula1WidgetData{}, err
		}
		data.WccOrder = wccOrder
	}

	return data, nil
}

func getNextSession(sessionType string) (Session, error) {
	response, _ := http.NewRequest("GET", NextRaceApi, nil)

	nextRace, err := decodeJsonFromRequest[formula1NextRaceHttpResponse](defaultHTTPClient, response)

	if err != nil {
		return Session{}, err
	}

	var session DatetimeWithTitle

	switch sessionType {
	case "all":
		session, err = getNextFutureDate(
			[]DatetimeWithTitle{
				{Datetime: nextRace.Race[0].Schedule.Fp1, Title: "Free Practice 1"},
				{Datetime: nextRace.Race[0].Schedule.Fp2, Title: "Free Practice 2"},
				{Datetime: nextRace.Race[0].Schedule.Fp3, Title: "Free Practice 3"},
				{Datetime: nextRace.Race[0].Schedule.SprintQualy, Title: "Sprint Qualifying"},
				{Datetime: nextRace.Race[0].Schedule.SprintRace, Title: "Sprint Race"},
				{Datetime: nextRace.Race[0].Schedule.Qualy, Title: "Qualifying"},
				{Datetime: nextRace.Race[0].Schedule.Race, Title: "Grand Prix Race"},
			},
		)
	case "major":
		session, err = getNextFutureDate([]DatetimeWithTitle{
			{Datetime: nextRace.Race[0].Schedule.Qualy, Title: "Qualifying"},
			{Datetime: nextRace.Race[0].Schedule.Race, Title: "Grand Prix Race"},
		})
	case "race":
	default:
		session = DatetimeWithTitle{Datetime: nextRace.Race[0].Schedule.Race, Title: "Grand Prix Race"}
	}
	sessionTime, _ := time.Parse(time.RFC3339, fmt.Sprintf("%sT%s", session.Datetime.Date, session.Datetime.Time))

	return Session{
		Title:       nextRace.Race[0].RaceName,
		Round:       nextRace.Race[0].Round,
		SessionName: session.Title,
		Unix:        sessionTime.Local().Unix(),
		DateString:  sessionTime.Local().Format(DateFormatString),
		Circuit:     nextRace.Race[0].Circuit.CircuitName,
		Country:     nextRace.Race[0].Circuit.Country,
		City:        nextRace.Race[0].Circuit.City,
	}, nil
}

func getLastRaceResults() (LastRace, error) {
	response, _ := http.NewRequest("GET", LastRaceApi, nil)
	lastRace, err := decodeJsonFromRequest[formula1LastRaceHttpResponse](defaultHTTPClient, response)

	if err != nil {
		return LastRace{}, err
	}

	lastRaceDateTime, _ := time.Parse(time.RFC3339, fmt.Sprintf("%sT%s", lastRace.Races.Date, lastRace.Races.Time))

	result := LastRace{
		RaceName:   lastRace.Races.RaceName,
		Round:      lastRace.Races.Round,
		DateString: lastRaceDateTime.Local().Format(DateFormatString),
		Results:    lastRace.Races.Results[:3],
	}

	return result, nil
}

func getWdcOrder() ([]Classification, error) {
	response, _ := http.NewRequest("GET", DriversChampionshipApi, nil)
	wdcOrder, err := decodeJsonFromRequest[formula1WdcHttpResponse](defaultHTTPClient, response)

	if err != nil {
		return []Classification{}, err
	}

	return wdcOrder.Drivers_Championship[:3], nil
}

func getWccOrder() ([]Classification, error) {
	response, _ := http.NewRequest("GET", ConstructorsChampionshipApi, nil)
	wccOrder, err := decodeJsonFromRequest[formula1WccHttpResponse](defaultHTTPClient, response)

	if err != nil {
		return []Classification{}, err
	}

	return wccOrder.Constructors_Championship[:3], nil
}

func getNextFutureDate(datetimesWithTitles []DatetimeWithTitle) (DatetimeWithTitle, error) {
	if len(datetimesWithTitles) == 0 {
		return DatetimeWithTitle{}, errors.New("empty datetime slice")
	}

	now := time.Now()
	futureDatetimeWithTitle := datetimesWithTitles[0]
	parsedFutureTime, err := time.Parse(time.RFC3339, fmt.Sprintf("%sT%s", futureDatetimeWithTitle.Datetime.Date, futureDatetimeWithTitle.Datetime.Time))
	if err != nil {
		return DatetimeWithTitle{}, fmt.Errorf("error parsing initial datetime: %v", err)
	}

	for _, datetimeWithTitle := range datetimesWithTitles {
		parsedTime, err := time.Parse(time.RFC3339, fmt.Sprintf("%sT%s", datetimeWithTitle.Datetime.Date, datetimeWithTitle.Datetime.Time))
		if err != nil {
			continue
		}

		if parsedTime.After(now) && parsedTime.Before(parsedFutureTime) {
			futureDatetimeWithTitle = datetimeWithTitle
			parsedFutureTime = parsedTime
		}
	}

	return futureDatetimeWithTitle, nil
}
