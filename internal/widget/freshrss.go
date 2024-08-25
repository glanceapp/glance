package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type FreshRSS struct {
	widgetBase      `yaml:",inline"`
	FeedRequests    []feed.RSSFeedRequest `yaml:"feeds"`
	Style           string                `yaml:"style"`
	ThumbnailHeight float64               `yaml:"thumbnail-height"`
	CardHeight      float64               `yaml:"card-height"`
	Items           feed.RSSFeedItems     `yaml:"-"`
	Limit           int                   `yaml:"limit"`
	CollapseAfter   int                   `yaml:"collapse-after"`
	FreshRSSUrl     string                `yaml:"freshrss-url"`
	FreshRSSUser    string                `yaml:"freshrss-user"`
	FreshRSSApiPass string                `yaml:"freshrss-api-pass"`
}

func (widget *FreshRSS) Initialize() error {
	widget.withTitle("FreshRSS Feed").withCacheDuration(1 * time.Hour)

	if widget.Limit <= 0 {
		widget.Limit = 25
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	if widget.ThumbnailHeight < 0 {
		widget.ThumbnailHeight = 0
	}

	if widget.CardHeight < 0 {
		widget.CardHeight = 0
	}

	return nil
}

func (widget *FreshRSS) Update(ctx context.Context) {

	var items feed.RSSFeedItems
	var err error

	items, err = feed.GetItemsFromFreshRssFeeds(widget.FreshRSSUrl, widget.FreshRSSUser, widget.FreshRSSApiPass)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(items) > widget.Limit {
		items = items[:widget.Limit]
	}

	widget.Items = items
}

func (widget *FreshRSS) Render() template.HTML {
	if widget.Style == "horizontal-cards" {
		return widget.render(widget, assets.RSSHorizontalCardsTemplate)
	}

	if widget.Style == "horizontal-cards-2" {
		return widget.render(widget, assets.RSSHorizontalCards2Template)
	}

	return widget.render(widget, assets.RSSListTemplate)
}
