package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type Releases struct {
	widgetBase          `yaml:",inline"`
	Releases            feed.AppReleases  `yaml:"-"`
	Repositories        []string          `yaml:"repositories"`
	Token               OptionalEnvString `yaml:"token"`
	Limit               int               `yaml:"limit"`
	CollapseAfter       int               `yaml:"collapse-after"`
	ReleasesSearchLimit int               `yaml:"releases-search-limit"`
	Starred             bool              `yaml:"starred"`
}

func (widget *Releases) Initialize() error {
	widget.withTitle("Releases").withCacheDuration(2 * time.Hour)

	if widget.Limit <= 0 {
		widget.Limit = 10
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	return nil
}

func (widget *Releases) Update(ctx context.Context) {
	var err error
	var releases []feed.AppRelease

	if widget.ReleasesSearchLimit <= 0 {
		widget.ReleasesSearchLimit = 10
	}

	if widget.Starred {
		releases, err = feed.FetchStarredRepositoriesReleasesFromGithub(string(widget.Token), widget.ReleasesSearchLimit)
	} else {
		releases, err = feed.FetchLatestReleasesFromGithub(widget.Repositories, string(widget.Token), widget.ReleasesSearchLimit)
	}

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(releases) > widget.Limit {
		releases = releases[:widget.Limit]
	}

	widget.Releases = releases
}

func (widget *Releases) Render() template.HTML {
	return widget.render(widget, assets.ReleasesTemplate)
}
