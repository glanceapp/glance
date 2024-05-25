package widget

import (
	"context"
	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
	"html/template"
	"time"
)

type Bilibili struct {
	widgetBase `yaml:",inline"`
	Videos     feed.Videos `yaml:"-"`
	Style      string      `yaml:"style"`
	UidList    []int       `yaml:"uidList"`
	Limit      int         `yaml:"limit"`
}

var _ Widget = (*Bilibili)(nil)

func (widget *Bilibili) Initialize() error {
	widget.withTitle("Bilibili").withCacheDuration(time.Hour)

	if widget.Limit <= 0 {
		widget.Limit = 25
	}
	return nil
}

func (widget *Bilibili) Update(ctx context.Context) {
	videos, err := feed.FetchBilibiliUploads(widget.UidList)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(videos) > widget.Limit {
		videos = videos[:widget.Limit]
	}

	widget.Videos = videos
}

func (widget *Bilibili) Render() template.HTML {
	if widget.Style == "grid-cards" {
		return widget.render(widget, assets.VideosGridTemplate)
	}

	return widget.render(widget, assets.VideosTemplate)
}
