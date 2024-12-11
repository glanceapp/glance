package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type TwitchChannels struct {
	widgetBase      	`yaml:",inline"`
	ChannelsRequest 	[]string             `yaml:"channels"`
	Channels        	[]feed.TwitchChannel `yaml:"-"`
	Groups        		map[string][]feed.TwitchChannel `yaml:"-"`
	CollapseAfter   	int                  `yaml:"collapse-after"`
	CollapseAfterRows 	int         		 `yaml:"collapse-after-rows"`
	Style           	string      		 `yaml:"style"`
	SortBy          	string               `yaml:"sort-by"`
	ShowOffline     	bool        		 `yaml:"show-offline"`
}

func (widget *TwitchChannels) Initialize() error {
	widget.
		withTitle("Twitch Channels").
		withTitleURL("https://www.twitch.tv/directory/following").
		withCacheDuration(time.Minute * 10)

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	if widget.CollapseAfterRows == 0 || widget.CollapseAfterRows < -1 {
		widget.CollapseAfterRows = 4
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
	
	if widget.Style == "grid-cards" {
		groupedChannels := channels.GroupByLive()
		widget.Groups = groupedChannels
	} 

	widget.Channels = channels
}

func (widget *TwitchChannels) Render() template.HTML {
	if widget.Style == "grid-cards" {
		return widget.render(widget, assets.TwitchChannelsGridTemplate)
	}
	return widget.render(widget, assets.TwitchChannelsTemplate)
}
