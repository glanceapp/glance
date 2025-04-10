package glance

import (
	"context"
	"html/template"
	"time"
)

var countTimerWidgetTemplate = mustParseTemplate("count-timer.html", "widget-base.html")

type countTimerWidget struct {
	widgetBase    `yaml:",inline"`
	cachedHTML    template.HTML `yaml:"-"`
	EventTitle    string        `yaml:"event-title"`
	TargetDate    time.Time     `yaml:"date"`
	Href          string        `yaml:"href"`
	RenderedTitle string        `yaml:"-"`
	DiffSeconds   int           `yaml:"-"`
	Days          int           `yaml:"-"`
	Hours         int           `yaml:"-"`
	Minutes       int           `yaml:"-"`
	Seconds       int           `yaml:"-"`
}

func (w *countTimerWidget) update(ctx context.Context) {
	now := time.Now()
	target := w.TargetDate

	diff := target.Sub(now)
	if diff < 0 {
		w.RenderedTitle = w.EventTitle + " ⋅ PAST"
		diff = -diff
	} else {
		w.RenderedTitle = w.EventTitle + " ⋅ FUTURE"
	}
	if w.EventTitle == "" {
		w.RenderedTitle = w.Title
	}
	w.Days = int(diff.Hours()) / 24
	w.Hours = int(diff.Hours()) % 24
	w.Minutes = int(diff.Minutes()) % 60
	w.Seconds = int(diff.Seconds()) % 60

	w.cachedHTML = w.renderTemplate(w, countTimerWidgetTemplate)
}

func (w *countTimerWidget) initialize() error {
	w.update(context.Background())
	w.withTitle(w.RenderedTitle).withError(nil)
	return nil
}

func (w *countTimerWidget) Render() template.HTML {
	w.update(context.TODO())
	return w.cachedHTML
}
