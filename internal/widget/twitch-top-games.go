package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type TwitchGames struct {
	widgetBase    `yaml:",inline"`
	Categories    []feed.TwitchCategory `yaml:"-"`
	Exclude       []string              `yaml:"exclude"`
	Limit         int                   `yaml:"limit"`
	CollapseAfter int                   `yaml:"collapse-after"`
}

func (widget *TwitchGames) Initialize() error {
	widget.withTitle("Top games on Twitch").withCacheDuration(time.Minute * 10)

	if widget.Limit <= 0 {
		widget.Limit = 10
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	return nil
}

func (widget *TwitchGames) Update(ctx context.Context) {
	categories, err := feed.FetchTopGamesFromTwitch(widget.Exclude, widget.Limit)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.Categories = categories
}

func (widget *TwitchGames) Render() template.HTML {
	return widget.render(widget, assets.TwitchGamesListTemplate)
}
