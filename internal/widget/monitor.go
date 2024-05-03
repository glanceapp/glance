package widget

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

func statusCodeToText(status int) string {
	if status == 200 {
		return "OK"
	}
	if status == 404 {
		return "Not Found"
	}
	if status == 403 {
		return "Forbidden"
	}
	if status == 401 {
		return "Unauthorized"
	}
	if status >= 400 {
		return "Client Error"
	}
	if status >= 500 {
		return "Server Error"
	}

	return strconv.Itoa(status)
}

func statusCodeToStyle(status int) string {
	if status == 200 {
		return "good"
	}

	return "bad"
}

type Monitor struct {
	widgetBase `yaml:",inline"`
	Sites      []struct {
		Title       string           `yaml:"title"`
		Url         string           `yaml:"url"`
		IconUrl     string           `yaml:"icon"`
		SameTab     bool             `yaml:"same-tab"`
		Status      *feed.SiteStatus `yaml:"-"`
		StatusText  string           `yaml:"-"`
		StatusStyle string           `yaml:"-"`
	} `yaml:"sites"`
}

func (widget *Monitor) Initialize() error {
	widget.withTitle("Monitor").withCacheDuration(5 * time.Minute)

	return nil
}

func (widget *Monitor) Update(ctx context.Context) {
	requests := make([]*http.Request, len(widget.Sites))

	for i := range widget.Sites {
		request, err := http.NewRequest("GET", widget.Sites[i].Url, nil)

		if err != nil {
			message := fmt.Errorf("failed to create http request for %s: %s", widget.Sites[i].Url, err)
			widget.withNotice(message)
			continue
		}

		requests[i] = request
	}

	statuses, err := feed.FetchStatusesForRequests(requests)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	for i := range widget.Sites {
		site := &widget.Sites[i]
		status := &statuses[i]

		site.Status = status

		if !status.TimedOut {
			site.StatusText = statusCodeToText(status.Code)
			site.StatusStyle = statusCodeToStyle(status.Code)
		}
	}
}

func (widget *Monitor) Render() template.HTML {
	return widget.render(widget, assets.MonitorTemplate)
}
