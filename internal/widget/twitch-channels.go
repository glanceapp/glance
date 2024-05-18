package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type TwitchChannels struct {
	widgetBase      `yaml:",inline"`
	ChannelsRequest []string             `yaml:"channels"`
	Channels        []feed.TwitchChannel `yaml:"-"`
	CollapseAfter   int                  `yaml:"collapse-after"`
	SortBy          string               `yaml:"sort-by"`
}

func (widget *TwitchChannels) Initialize() error {
	widget.withTitle("Twitch Channels").withCacheDuration(time.Minute * 10)

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	if widget.SortBy != "viewers" && widget.SortBy != "live" {
		widget.SortBy = "viewers"
	}

	return nil
}

func (widget *TwitchChannels) Update(ctx context.Context) {
	channels, err := feed.FetchChannelsFromTwitch(widget.ChannelsRequest)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if widget.SortBy == "viewers" {
		channels.SortByViewers()
	} else if widget.SortBy == "live" {
		channels.SortByLive()
	}

	widget.Channels = channels
}

func (widget *TwitchChannels) Render() template.HTML {
	return widget.render(widget, assets.TwitchChannelsTemplate)
}
