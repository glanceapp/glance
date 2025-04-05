package glance

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"time"
)

var (
	monitorWidgetTemplate        = mustParseTemplate("monitor.html", "widget-base.html")
	monitorWidgetCompactTemplate = mustParseTemplate("monitor-compact.html", "widget-base.html")
)

type monitorWidget struct {
	widgetBase `yaml:",inline"`
	Sites      []struct {
		*SiteStatusRequest `yaml:",inline"`
		Status             *siteStatus     `yaml:"-"`
		Title              string          `yaml:"title"`
		Icon               customIconField `yaml:"icon"`
		SameTab            bool            `yaml:"same-tab"`
		StatusText         string          `yaml:"-"`
		StatusStyle        string          `yaml:"-"`
		AltStatusCodes     []int           `yaml:"alt-status-codes"`
	} `yaml:"sites"`
	Style           string `yaml:"style"`
	ShowFailingOnly bool   `yaml:"show-failing-only"`
	HasFailing      bool   `yaml:"-"`
}

func (widget *monitorWidget) initialize() error {
	widget.withTitle("Monitor").withCacheDuration(5 * time.Minute)

	return nil
}

func (widget *monitorWidget) update(ctx context.Context) {
	requests := make([]*SiteStatusRequest, len(widget.Sites))

	for i := range widget.Sites {
		requests[i] = widget.Sites[i].SiteStatusRequest
	}

	statuses, err := fetchStatusForSites(requests)

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

func (widget *monitorWidget) Render() template.HTML {
	if widget.Style == "compact" {
		return widget.renderTemplate(widget, monitorWidgetCompactTemplate)
	}

	return widget.renderTemplate(widget, monitorWidgetTemplate)
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

type SiteStatusRequest struct {
	URL           string `yaml:"url"`
	CheckURL      string `yaml:"check-url"`
	AllowInsecure bool   `yaml:"allow-insecure"`
}

type siteStatus struct {
	Code         int
	TimedOut     bool
	ResponseTime time.Duration
	Error        error
}

func fetchSiteStatusTask(statusRequest *SiteStatusRequest) (siteStatus, error) {
	var url string
	if statusRequest.CheckURL != "" {
		url = statusRequest.CheckURL
	} else {
		url = statusRequest.URL
	}
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return siteStatus{
			Error: err,
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	request = request.WithContext(ctx)
	requestSentAt := time.Now()
	var response *http.Response

	if !statusRequest.AllowInsecure {
		response, err = defaultHTTPClient.Do(request)
	} else {
		response, err = defaultInsecureHTTPClient.Do(request)
	}

	status := siteStatus{ResponseTime: time.Since(requestSentAt)}

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			status.TimedOut = true
		}

		status.Error = err
		return status, nil
	}

	defer response.Body.Close()

	status.Code = response.StatusCode

	return status, nil
}

func fetchStatusForSites(requests []*SiteStatusRequest) ([]siteStatus, error) {
	job := newJob(fetchSiteStatusTask, requests).withWorkers(20)
	results, _, err := workerPoolDo(job)
	if err != nil {
		return nil, err
	}

	return results, nil
}
