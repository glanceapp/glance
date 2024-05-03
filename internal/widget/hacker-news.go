package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type HackerNews struct {
	widgetBase          `yaml:",inline"`
	Posts               feed.ForumPosts `yaml:"-"`
	Limit               int             `yaml:"limit"`
	CollapseAfter       int             `yaml:"collapse-after"`
	CommentsUrlTemplate string          `yaml:"comments-url-template"`
	ShowThumbnails      bool            `yaml:"-"`
}

func (widget *HackerNews) Initialize() error {
	widget.withTitle("Hacker News").withCacheDuration(30 * time.Minute)

	if widget.Limit <= 0 {
		widget.Limit = 15
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	return nil
}

func (widget *HackerNews) Update(ctx context.Context) {
	posts, err := feed.FetchHackerNewsTopPosts(40, widget.CommentsUrlTemplate)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	posts.CalculateEngagement()
	posts.SortByEngagement()

	if widget.Limit < len(posts) {
		posts = posts[:widget.Limit]
	}

	widget.Posts = posts
}

func (widget *HackerNews) Render() template.HTML {
	return widget.render(widget, assets.ForumPostsTemplate)
}
