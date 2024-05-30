package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type ChangeDetection struct {
	widgetBase       `yaml:",inline"`
	ChangeDetections feed.ChangeDetectionWatches `yaml:"-"`
	WatchUUIDs       []string                    `yaml:"watches"`
	InstanceURL      string                      `yaml:"instance-url"`
	Token            OptionalEnvString           `yaml:"token"`
	Limit            int                         `yaml:"limit"`
	CollapseAfter    int                         `yaml:"collapse-after"`
}

func (widget *ChangeDetection) Initialize() error {
	widget.withTitle("Change Detection").withCacheDuration(1 * time.Hour)

	if widget.Limit <= 0 {
		widget.Limit = 10
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	if widget.InstanceURL == "" {
		widget.InstanceURL = "https://www.changedetection.io"
	}

	return nil
}

func (widget *ChangeDetection) Update(ctx context.Context) {
	if len(widget.WatchUUIDs) == 0 {
		uuids, err := feed.FetchWatchUUIDsFromChangeDetection(widget.InstanceURL, string(widget.Token))

		if !widget.canContinueUpdateAfterHandlingErr(err) {
			return
		}

		widget.WatchUUIDs = uuids
	}

	watches, err := feed.FetchWatchesFromChangeDetection(widget.InstanceURL, widget.WatchUUIDs, string(widget.Token))

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(watches) > widget.Limit {
		watches = watches[:widget.Limit]
	}

	widget.ChangeDetections = watches
}

func (widget *ChangeDetection) Render() template.HTML {
	return widget.render(widget, assets.ChangeDetectionTemplate)
}
