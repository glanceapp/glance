package glance

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"sync"
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
		URL                string          `yaml:"-"`
		ErrorURL           string          `yaml:"error-url"`
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
	println("DEBUG: Starting update, number of sites:", len(widget.Sites))

	var wg sync.WaitGroup
	statuses := make([]siteStatus, len(widget.Sites))
	var mu sync.Mutex
	var fetchErr error

	// Launch goroutines for each site
	for i := range widget.Sites {
		println("DEBUG: Checking site", i, "- Request is nil:", widget.Sites[i].SiteStatusRequest == nil)

		if widget.Sites[i].SiteStatusRequest == nil {
			println("DEBUG: Skipping site", i, "- nil request")
			continue
		}

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			println("DEBUG: Fetching status for site", idx)
			status, err := fetchSiteStatusTask(widget.Sites[idx].SiteStatusRequest)
			if err != nil && fetchErr == nil {
				mu.Lock()
				fetchErr = err
				mu.Unlock()
			}
			println("DEBUG: Site", idx, "- Status Code:", status.Code, "Error:", status.Error)

			mu.Lock()
			statuses[idx] = status
			mu.Unlock()
		}(i)
	}

	// Wait for all goroutines to complete
	println("DEBUG: Waiting for all goroutines to complete")
	wg.Wait()
	println("DEBUG: All goroutines completed")

	if !widget.canContinueUpdateAfterHandlingErr(fetchErr) {
		println("DEBUG: Update aborted due to error:", fetchErr)
		return
	}

	widget.HasFailing = false

	for i := range widget.Sites {
		site := &widget.Sites[i]
		status := &statuses[i]
		site.Status = status

		println("DEBUG: Processing site", i, "- Code:", status.Code, "Error:", status.Error)

		if !slices.Contains(site.AltStatusCodes, status.Code) && (status.Code >= 400 || status.Error != nil) {
			widget.HasFailing = true
			println("DEBUG: Site", i, "marked as failing")
		}

		if status.Error != nil && site.ErrorURL != "" {
			site.URL = site.ErrorURL
			println("DEBUG: Site", i, "using error URL:", site.ErrorURL)
		} else {
			site.URL = site.DefaultURL
			println("DEBUG: Site", i, "using default URL:", site.DefaultURL)
		}

		site.StatusText = statusCodeToText(status.Code, site.AltStatusCodes)
		site.StatusStyle = statusCodeToStyle(status.Code, site.AltStatusCodes)
		println("DEBUG: Site", i, "- StatusText:", site.StatusText, "StatusStyle:", site.StatusStyle)
	}

	println("DEBUG: Update complete - HasFailing:", widget.HasFailing)
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
	if status >= 500 {
		return "Server Error"
	}
	if status >= 400 {
		return "Client Error"
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
	DefaultURL    string        `yaml:"url"`
	CheckURL      string        `yaml:"check-url"`
	AllowInsecure bool          `yaml:"allow-insecure"`
	Timeout       durationField `yaml:"timeout"`
	BasicAuth     struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"basic-auth"`
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
		url = statusRequest.DefaultURL
	}

	timeout := ternary(statusRequest.Timeout > 0, time.Duration(statusRequest.Timeout), 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return siteStatus{
			Error: err,
		}, nil
	}

	if statusRequest.BasicAuth.Username != "" || statusRequest.BasicAuth.Password != "" {
		request.SetBasicAuth(statusRequest.BasicAuth.Username, statusRequest.BasicAuth.Password)
	}

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
