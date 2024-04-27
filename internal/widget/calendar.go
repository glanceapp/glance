package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type Calendar struct {
	widgetBase `yaml:",inline"`
	Calendar   *feed.Calendar
}

func (widget *Calendar) Initialize() error {
	widget.withTitle("Calendar").withCacheOnTheHour()

	return nil
}

func (widget *Calendar) Update(ctx context.Context) {
	widget.Calendar = feed.NewCalendar(time.Now())
	widget.withError(nil).scheduleNextUpdate()
}

func (widget *Calendar) Render() template.HTML {
	return widget.render(widget, assets.CalendarTemplate)
}
