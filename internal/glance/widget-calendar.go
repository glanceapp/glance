package glance

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
)

var calendarWidgetTemplate = mustParseTemplate("calendar.html", "widget-base.html")

var calendarWeekdaysToInt = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
}

type calendarWidget struct {
	widgetBase     `yaml:",inline"`
	FirstDayOfWeek string        `yaml:"first-day-of-week"`
	Ics            []string      `yaml:"ics"`
	FirstDay       int           `yaml:"-"`
	cachedHTML     template.HTML `yaml:"-"`
	Events         string        `yaml:"events"`
}

type calendarEvent struct {
	Date time.Time
	Name string
}

func (widget *calendarWidget) initialize() error {
	widget.withTitle("Calendar").withError(nil)

	if widget.FirstDayOfWeek == "" {
		widget.FirstDayOfWeek = "monday"
	} else if _, ok := calendarWeekdaysToInt[widget.FirstDayOfWeek]; !ok {
		return errors.New("invalid first day of week")
	}

	var events []*ics.VEvent
	var widgetEvents []calendarEvent
	for _, url := range widget.Ics {
		newEvents, err := ReadPublicIcs(url)
		if err != nil {
			fmt.Println(err)
		}
		events = append(events, newEvents...)
	}

	for _, event := range events {
		startDate, _ := event.GetStartAt()
		e := calendarEvent{
			Date: startDate,
			Name: event.GetProperty("SUMMARY").Value,
		}
		widgetEvents = append(widgetEvents, e)

	}
	jsonBytes, err := json.Marshal(widgetEvents)
	if err != nil {
		panic(err)
	}
	widget.Events = string(jsonBytes)
	widget.FirstDay = int(calendarWeekdaysToInt[widget.FirstDayOfWeek])
	widget.cachedHTML = widget.renderTemplate(widget, calendarWidgetTemplate)

	return nil
}

func (widget *calendarWidget) Render() template.HTML {
	return widget.cachedHTML
}

func ParseEventsFromFile(file string) []*ics.VEvent {
	eventString, err := os.ReadFile(file)
	if err != nil {
		log.Panic(err)
	}
	cal, err := ics.ParseCalendar(strings.NewReader(string(eventString)))
	if err != nil {
		log.Panic(err)
	}
	events := cal.Events()
	return events
}
func ReadPublicIcs(url string) ([]*ics.VEvent, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	cal, err := ics.ParseCalendar(response.Body)
	if err != nil {
		return nil, err
	}
	events := cal.Events()
	return events, nil
}
