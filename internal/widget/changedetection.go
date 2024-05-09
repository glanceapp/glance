package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type ChangeDetections struct {
	widgetBase       `yaml:",inline"`
	ChangeDetections feed.ChangeWatches `yaml:"-"`
	RequestURL       string             `yaml:"request_url"`
	Watches          []string           `yaml:"watches"`
	Token            OptionalEnvString  `yaml:"token"`
	Limit            int                `yaml:"limit"`
	CollapseAfter    int                `yaml:"collapse-after"`
}

func (widget *ChangeDetections) Initialize() error {
	widget.withTitle("Changes").withCacheDuration(2 * time.Hour)

	if widget.Limit <= 0 {
		widget.Limit = 10
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	return nil
}

func (widget *ChangeDetections) Update(ctx context.Context) {
	watches, err := feed.FetchLatestDetectedChanges(widget.RequestURL, widget.Watches, string(widget.Token))

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(watches) > widget.Limit {
		watches = watches[:widget.Limit]
	}

	widget.ChangeDetections = watches
}

func (widget *ChangeDetections) Render() template.HTML {
	return widget.render(widget, assets.ChangesTemplate)
}
