package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type Lobsters struct {
	widgetBase     `yaml:",inline"`
	Posts          feed.ForumPosts `yaml:"-"`
	InstanceURL    string          `yaml:"instance-url"`
	CustomURL      string          `yaml:"custom-url"`
	Limit          int             `yaml:"limit"`
	CollapseAfter  int             `yaml:"collapse-after"`
	SortBy         string          `yaml:"sort-by"`
	Tags           []string        `yaml:"tags"`
	ShowThumbnails bool            `yaml:"-"`
}

func (widget *Lobsters) Initialize() error {
	widget.withTitle("Lobsters").withCacheDuration(time.Hour)

	if widget.InstanceURL == "" {
		widget.withTitleURL("https://lobste.rs")
	} else {
		widget.withTitleURL(widget.InstanceURL)
	}

	if widget.SortBy == "" || (widget.SortBy != "hot" && widget.SortBy != "new") {
		widget.SortBy = "hot"
	}

	if widget.Limit <= 0 {
		widget.Limit = 15
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	return nil
}

func (widget *Lobsters) Update(ctx context.Context) {
	posts, err := feed.FetchLobstersPosts(widget.CustomURL, widget.InstanceURL, widget.SortBy, widget.Tags)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if widget.Limit < len(posts) {
		posts = posts[:widget.Limit]
	}

	widget.Posts = posts
}

func (widget *Lobsters) Render() template.HTML {
	return widget.render(widget, assets.ForumPostsTemplate)
}
