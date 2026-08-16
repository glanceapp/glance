package glance

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"strings"
	"sync"
	"time"
)

var trueNASWidgetTemplate = mustParseTemplate("truenas.html", "widget-base.html")

type trueNASWidget struct {
	widgetBase `yaml:",inline"`

	URL           string               `yaml:"url"`
	APIKey        string               `yaml:"api-key"`
	AllowInsecure bool                 `yaml:"allow-insecure"`
	LoadAverage   float64              `yaml:"-"`
	BootTime      time.Time            `yaml:"-"`
	PendingAlerts int                  `yaml:"-"`
	Pools         []trueNASPool        `yaml:"-"`
	requests      [3]*CustomAPIRequest `yaml:"-"`
}

var trueNASResources = [3]struct{ path, name string }{
	{"/pool", "pools"}, {"/system/info", "system information"}, {"/alert/list", "alerts"},
}

func (widget *trueNASWidget) initialize() error {
	widget.
		withTitle("TrueNAS").
		withTitleURL(strings.TrimRight(widget.URL, "/")).
		withCacheDuration(5 * time.Minute)

	if widget.URL == "" || widget.APIKey == "" {
		return errors.New("url and api-key are required")
	}

	baseURL := strings.TrimRight(widget.URL, "/") + "/api/v2.0"
	for i, resource := range trueNASResources {
		widget.requests[i] = &CustomAPIRequest{
			URL:           baseURL + resource.path,
			AllowInsecure: widget.AllowInsecure,
			Headers:       map[string]string{"Authorization": "Bearer " + widget.APIKey},
		}
		if err := widget.requests[i].initialize(); err != nil {
			return fmt.Errorf("initializing %s request: %w", resource.name, err)
		}
	}
	return nil
}

func (widget *trueNASWidget) update(ctx context.Context) {
	var responses [3]*customAPIResponseData
	var errs [3]error
	var waitGroup sync.WaitGroup
	for i, request := range widget.requests {
		waitGroup.Go(func() {
			responses[i], errs[i] = fetchCustomAPIResponse(ctx, request)
		})
	}
	waitGroup.Wait()

	var err error
	for i := range errs {
		if errs[i] != nil {
			err = fmt.Errorf("fetching %s: %w", trueNASResources[i].name, errs[i])
			break
		}
	}
	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.LoadAverage = responses[1].JSON.Float("loadavg.0")
	widget.BootTime = time.Now().Add(-time.Duration(responses[1].JSON.Float("uptime_seconds") * float64(time.Second)))
	widget.Pools = widget.Pools[:0]
	for _, pool := range responses[0].JSON.Array("") {
		widget.Pools = append(widget.Pools, trueNASPool{
			Name: pool.String("name"), Healthy: pool.Bool("healthy"),
		})
	}
	widget.PendingAlerts = 0
	for _, alert := range responses[2].JSON.Array("") {
		if !alert.Bool("dismissed") {
			widget.PendingAlerts++
		}
	}
}

func (widget *trueNASWidget) Render() template.HTML {
	return widget.renderTemplate(widget, trueNASWidgetTemplate)
}

type trueNASPool struct {
	Name    string
	Healthy bool
}
