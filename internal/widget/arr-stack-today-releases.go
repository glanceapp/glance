package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type ArrReleases struct {
	widgetBase `yaml:",inline"`
	Releases   feed.ArrReleases `yaml:"-"`
	Sonarr     struct {
		Enable   bool   `yaml:"enable"`
		Endpoint string `yaml:"endpoint"`
		ApiKey   string `yaml:"apikey"`
	}
	Radarr struct {
		Enable   bool   `yaml:"enable"`
		Endpoint string `yaml:"endpoint"`
		ApiKey   string `yaml:"apikey"`
	}
	CollapseAfter int           `yaml:"collapse-after"`
	CacheDuration time.Duration `yaml:"cache-duration"`
}

func (widget *ArrReleases) Initialize() error {
	widget.withTitle("Releasing Today")

	// Set cache duration
	if widget.CacheDuration == 0 {
		widget.CacheDuration = time.Minute * 5
	}
	widget.withCacheDuration(widget.CacheDuration)

	// Set collapse after default value
	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	return nil
}

func (widget *ArrReleases) Update(ctx context.Context) {
	releases, err := feed.FetchReleasesFromArrStack(widget.Sonarr, widget.Radarr)
	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.Releases = releases
}

func (widget *ArrReleases) Render() template.HTML {
	return widget.render(widget, assets.ArrReleasesTemplate)
}
