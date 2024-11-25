package feed

import "time"

// TODO: very inflexible, refactor to allow more customizability
// TODO: allow changing between showing the previous and next week and the entire month
func NewCalendar(now time.Time, startSunday bool) *Calendar {
	year, week := now.ISOWeek()
	weekday := now.Weekday()
	if !startSunday {
		weekday = (weekday + 6) % 7 // Shift Monday to 0
	}

	currentMonthDays := daysInMonth(now.Month(), year)

	var previousMonthDays int

	if previousMonthNumber := now.Month() - 1; previousMonthNumber < 1 {
		previousMonthDays = daysInMonth(12, year-1)
	} else {
		previousMonthDays = daysInMonth(previousMonthNumber, year)
	}

	startDaysFrom := now.Day() - int(weekday) - 7

	days := make([]int, 21)

	for i := 0; i < 21; i++ {
		day := startDaysFrom + i

		if day < 1 {
			day = previousMonthDays + day
		} else if day > currentMonthDays {
			day = day - currentMonthDays
		}

		days[i] = day
	}

	return &Calendar{
		CurrentDay:        now.Day(),
		CurrentWeekNumber: week,
		CurrentMonthName:  now.Month().String(),
		CurrentYear:       year,
		Days:              days,
	}
}

func daysInMonth(m time.Month, year int) int {
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
