package glance

import (
	"context"
	"errors"
	"html/template"
	"net/url"
	"strings"
	"time"
)

var tautulliWidgetTemplate = mustParseTemplate("tautulli.html", "widget-base.html")

type tautulliWidget struct {
	widgetBase `yaml:",inline"`

	URL           string            `yaml:"url"`
	APIKey        string            `yaml:"api-key"`
	AllowInsecure bool              `yaml:"allow-insecure"`
	StreamCount   int               `yaml:"-"`
	Sessions      []tautulliSession `yaml:"-"`
	request       *CustomAPIRequest `yaml:"-"`
}

func (widget *tautulliWidget) initialize() error {
	widget.
		withTitle("Tautulli").
		withTitleURL(strings.TrimRight(widget.URL, "/")).
		withCacheDuration(time.Minute)

	if widget.URL == "" || widget.APIKey == "" {
		return errors.New("url and api-key are required")
	}

	widget.request = &CustomAPIRequest{
		URL:           strings.TrimRight(widget.URL, "/") + "/api/v2",
		AllowInsecure: widget.AllowInsecure,
		Parameters:    queryParametersField{"apikey": {widget.APIKey}, "cmd": {"get_activity"}},
	}
	return widget.request.initialize()
}

func (widget *tautulliWidget) update(ctx context.Context) {
	response, err := fetchCustomAPIResponse(ctx, widget.request)
	if err != nil {
		message := strings.ReplaceAll(err.Error(), url.QueryEscape(widget.APIKey), "[redacted]")
		err = errors.New(strings.ReplaceAll(message, widget.APIKey, "[redacted]"))
	} else if response.JSON.String("response.result") != "success" {
		message := response.JSON.String("response.message")
		if message == "" {
			message = "Tautulli API request failed"
		}
		err = errors.New(message)
	}
	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.StreamCount = response.JSON.Int("response.data.stream_count")
	widget.Sessions = widget.Sessions[:0]
	for _, session := range response.JSON.Array("response.data.sessions") {
		widget.Sessions = append(widget.Sessions, tautulliSession{
			Title:           session.String("full_title"),
			FriendlyName:    session.String("friendly_name"),
			ProgressPercent: max(0, min(100, session.Int("progress_percent"))),
		})
	}
}

func (widget *tautulliWidget) Render() template.HTML {
	return widget.renderTemplate(widget, tautulliWidgetTemplate)
}

type tautulliSession struct {
	Title           string
	FriendlyName    string
	ProgressPercent int
}
