package widget

import (
	"context"
	"errors"
	"html/template"
	"strings"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type Reddit struct {
	widgetBase          `yaml:",inline"`
	Posts               feed.ForumPosts `yaml:"-"`
	Subreddit           string          `yaml:"subreddit"`
	Style               string          `yaml:"style"`
	ShowThumbnails      bool            `yaml:"show-thumbnails"`
	SortBy              string          `yaml:"sort-by"`
	TopPeriod           string          `yaml:"top-period"`
	Search              string          `yaml:"search"`
	ExtraSortBy         string          `yaml:"extra-sort-by"`
	CommentsUrlTemplate string          `yaml:"comments-url-template"`
	Limit               int             `yaml:"limit"`
	CollapseAfter       int             `yaml:"collapse-after"`
	RequestUrlTemplate  string          `yaml:"request-url-template"`
}

func (widget *Reddit) Initialize() error {
	if widget.Subreddit == "" {
		return errors.New("no subreddit specified")
	}

	if widget.Limit <= 0 {
		widget.Limit = 15
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	if !isValidRedditSortType(widget.SortBy) {
		widget.SortBy = "hot"
	}

	if !isValidRedditTopPeriod(widget.TopPeriod) {
		widget.TopPeriod = "day"
	}

	if widget.RequestUrlTemplate != "" {
		if !strings.Contains(widget.RequestUrlTemplate, "{REQUEST-URL}") {
			return errors.New("no `{REQUEST-URL}` placeholder specified")
		}
	}

	widget.withTitle("/r/" + widget.Subreddit).withCacheDuration(30 * time.Minute)

	return nil
}

func isValidRedditSortType(sortBy string) bool {
	return sortBy == "hot" ||
		sortBy == "new" ||
		sortBy == "top" ||
		sortBy == "rising"
}

func isValidRedditTopPeriod(period string) bool {
	return period == "hour" ||
		period == "day" ||
		period == "week" ||
		period == "month" ||
		period == "year" ||
		period == "all"
}

func (widget *Reddit) Update(ctx context.Context) {
	// TODO: refactor, use a struct to pass all of these
	posts, err := feed.FetchSubredditPosts(
		widget.Subreddit,
		widget.SortBy,
		widget.TopPeriod,
		widget.Search,
		widget.CommentsUrlTemplate,
		widget.RequestUrlTemplate,
	)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(posts) > widget.Limit {
		posts = posts[:widget.Limit]
	}

	if widget.ExtraSortBy == "engagement" {
		posts.CalculateEngagement()
		posts.SortByEngagement()
	}

	widget.Posts = posts
}

func (widget *Reddit) Render() template.HTML {
	if widget.Style == "horizontal-cards" {
		return widget.render(widget, assets.RedditCardsHorizontalTemplate)
	}

	if widget.Style == "vertical-cards" {
		return widget.render(widget, assets.RedditCardsVerticalTemplate)
	}

	return widget.render(widget, assets.ForumPostsTemplate)

}
