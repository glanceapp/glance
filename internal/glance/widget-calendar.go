package glance

import (
	"context"
	"fmt"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
)

var calendarWidgetTemplate = mustParseTemplate("calendar.html", "widget-base.html")

type CalendarEvent struct {
	StartedDay time.Time
	EventHover string
}
type CalendayDay struct {
	Day     int
	IsEvent bool
	Events  []CalendarEvent
}

type calendarWidget struct {
	widgetBase  `yaml:",inline"`
	Calendar    *calendar
	StartSunday bool `yaml:"start-sunday"`
	Icsurl      string
}

func (widget *calendarWidget) initialize() error {
	widget.withTitle("Calendar").withCacheOnTheHour()

	return nil
}

func (widget *calendarWidget) update(ctx context.Context) {
	widget.Calendar = newCalendar(time.Now(), widget.StartSunday, widget.Icsurl)
	fmt.Println(widget.Calendar.Days)
	widget.withError(nil).scheduleNextUpdate()
}

func (widget *calendarWidget) Render() template.HTML {
	return widget.renderTemplate(widget, calendarWidgetTemplate)
}

type calendar struct {
	CurrentDay        int
	CurrentWeekNumber int
	CurrentMonthName  string
	CurrentYear       int
	Days              []CalendayDay
	Icsurl            string `yaml:"icsurl"`
}

// TODO: very inflexible, refactor to allow more customizability
// TODO: allow changing between showing the previous and next week and the entire month
func newCalendar(now time.Time, startSunday bool, icsurl string) *calendar {
	year, week := now.Year(), int(now.Weekday())
	weekday := now.Weekday()
	if !startSunday {
		weekday = (weekday + 6) % 7 // Shift Monday to 0
	}

	currentMonthDays := daysInMonth(now.Month(), year)

	var previousMonthDays int

	if widget.FirstDayOfWeek == "" {
		widget.FirstDayOfWeek = "monday"
	} else if _, ok := calendarWeekdaysToInt[widget.FirstDayOfWeek]; !ok {
		return errors.New("invalid first day of week")
	}

	startDaysFrom := now.Day() - int(weekday) - 7

	days := make([]CalendayDay, 21)
	events, _ := ReadPublicIcs(icsurl)

	for i := 0; i < 21; i++ {
		day := startDaysFrom + i
		month := now.Month()

		if day < 1 {
			day = previousMonthDays + day
			month -= 1

		} else if day > currentMonthDays {
			day = day - currentMonthDays
			month += 1

		}
		if events != nil {
			for _, event := range events {
				var dayEvent CalendarEvent
				startAt, err := event.GetStartAt()
				if err != nil {
					log.Panic(err)
				}
				fmt.Println(year)
				// fmt.Println(startAt.Day() == day && startAt.Month() == month)
				if startAt.Day() == day && startAt.Month() == month && startAt.Year() == year {
					dayEvent.StartedDay = startAt
					dayEvent.EventHover = event.GetProperty("SUMMARY").Value
					days[i].IsEvent = true
					days[i].Events = append(days[i].Events, dayEvent)
				}
			}
		}
		days[i].Day = day
	}

	return &calendar{
		CurrentDay:        now.Day(),
		CurrentWeekNumber: week,
		CurrentMonthName:  now.Month().String(),
		CurrentYear:       year,
		Days:              days,
	}
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
