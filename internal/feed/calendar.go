package feed

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
)

// TODO: very inflexible, refactor to allow more customizability
// TODO: allow changing first day of week
// TODO: allow changing between showing the previous and next week and the entire month
func NewCalendar(now time.Time, icsurl string) *Calendar {
	year, week := now.ISOWeek()
	weekday := now.Weekday()

	if weekday == 0 {
		weekday = 7
	}

	currentMonthDays := daysInMonth(now.Month(), year)

	var previousMonthDays int

	if previousMonthNumber := now.Month() - 1; previousMonthNumber < 1 {
		previousMonthDays = daysInMonth(12, year-1)
	} else {
		previousMonthDays = daysInMonth(previousMonthNumber, year)
	}

	startDaysFrom := now.Day() - int(weekday+6)

	days := make([]CalendayDay, 21)
	events, _ := ReadPublicIcs(icsurl)
	for i := 0; i < 21; i++ {
		day := startDaysFrom + i
		month := now.Month()
		var dayEvent CalendarEvent

		if day < 1 {
			day = previousMonthDays + day
			month -= 1
		} else if day > currentMonthDays {
			day = day - currentMonthDays
			month += 1
		}
		if events != nil {
			for _, event := range events {
				startAt, err := event.GetStartAt()
				if err != nil {
					log.Panic(err)
				}
				if startAt.Day() == day && startAt.Month() == month && startAt.Year() == year {
					dayEvent.StartedDay = startAt
					dayEvent.EventHover = event.GetProperty("SUMMARY").Value
					days[i].IsEvent = true
				}
			}
		}
		days[i].Day = day
		days[i].Event = dayEvent
	}

	return &Calendar{
		CurrentDay:        now.Day(),
		CurrentWeekNumber: week,
		CurrentMonthName:  now.Month().String(),
		CurrentYear:       year,
		Days:              days,
	}
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

func daysInMonth(m time.Month, year int) int {
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
