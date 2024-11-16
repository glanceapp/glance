package widget

import (
	"context"
	"html/template"
	"slices"
	"strconv"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type Monitor struct {
	widgetBase `yaml:",inline"`
	Sites      []struct {
		*feed.SiteStatusRequest `yaml:",inline"`
		Status                  *feed.SiteStatus `yaml:"-"`
		Title                   string           `yaml:"title"`
		Icon                    CustomIcon       `yaml:"icon"`
		SameTab                 bool             `yaml:"same-tab"`
		StatusText              string           `yaml:"-"`
		StatusStyle             string           `yaml:"-"`
		AltStatusCodes          []int            `yaml:"alt-status-codes"`
	} `yaml:"sites"`
	Style           string `yaml:"style"`
	ShowFailingOnly bool   `yaml:"show-failing-only"`
	HasFailing      bool   `yaml:"-"`
}

func (widget *Monitor) Initialize() error {
	widget.withTitle("Monitor").withCacheDuration(5 * time.Minute)

	return nil
}

func (widget *Monitor) Update(ctx context.Context) {
	requests := make([]*feed.SiteStatusRequest, len(widget.Sites))

	for i := range widget.Sites {
		requests[i] = widget.Sites[i].SiteStatusRequest
	}

	statuses, err := feed.FetchStatusForSites(requests)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.HasFailing = false

	for i := range widget.Sites {
		site := &widget.Sites[i]
		status := &statuses[i]
		site.Status = status

		if !slices.Contains(site.AltStatusCodes, status.Code) && (status.Code >= 400 || status.TimedOut || status.Error != nil) {
			widget.HasFailing = true
		}

		if !status.TimedOut {
			site.StatusText = statusCodeToText(status.Code, site.AltStatusCodes)
			site.StatusStyle = statusCodeToStyle(status.Code, site.AltStatusCodes)
		}
	}
}

func (widget *Monitor) Render() template.HTML {
	if widget.Style == "compact" {
		return widget.render(widget, assets.MonitorCompactTemplate)
	}

	return widget.render(widget, assets.MonitorTemplate)
}

func statusCodeToText(status int, altStatusCodes []int) string {
	if status == 200 || slices.Contains(altStatusCodes, status) {
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

func statusCodeToStyle(status int, altStatusCodes []int) string {
	if status == 200 || slices.Contains(altStatusCodes, status) {
		return "ok"
	}

	return "error"
}
