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

type Releases struct {
	widgetBase      `yaml:",inline"`
	Releases        feed.AppReleases       `yaml:"-"`
	releaseRequests []*feed.ReleaseRequest `yaml:"-"`
	Repositories    []string               `yaml:"repositories"`
	Token           OptionalEnvString      `yaml:"token"`
	GitLabToken     OptionalEnvString      `yaml:"gitlab-token"`
	Limit           int                    `yaml:"limit"`
	CollapseAfter   int                    `yaml:"collapse-after"`
	ShowSourceIcon  bool                   `yaml:"show-source-icon"`
	Style           string                 `yaml:"style"`
}

func (widget *Releases) Initialize() error {
	widget.withTitle("Releases").withCacheDuration(2 * time.Hour)

	if widget.Limit <= 0 {
		widget.Limit = 10
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	var tokenAsString = widget.Token.String()
	var gitLabTokenAsString = widget.GitLabToken.String()

	for _, repository := range widget.Repositories {
		parts := strings.SplitN(repository, ":", 2)
		var request *feed.ReleaseRequest

		if len(parts) == 1 {
			request = &feed.ReleaseRequest{
				Source:     feed.ReleaseSourceGithub,
				Repository: repository,
			}

			if widget.Token != "" {
				request.Token = &tokenAsString
			}
		} else if len(parts) == 2 {
			if parts[0] == string(feed.ReleaseSourceGitlab) {
				request = &feed.ReleaseRequest{
					Source:     feed.ReleaseSourceGitlab,
					Repository: parts[1],
				}

				if widget.GitLabToken != "" {
					request.Token = &gitLabTokenAsString
				}
			} else if parts[0] == string(feed.ReleaseSourceDockerHub) {
				request = &feed.ReleaseRequest{
					Source:     feed.ReleaseSourceDockerHub,
					Repository: parts[1],
				}
			} else {
				return errors.New("invalid repository source " + parts[0])
			}
		}

		widget.releaseRequests = append(widget.releaseRequests, request)
	}

	return nil
}

func (widget *Releases) Update(ctx context.Context) {
	releases, err := feed.FetchLatestReleases(widget.releaseRequests)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(releases) > widget.Limit {
		releases = releases[:widget.Limit]
	}

	for i := range releases {
		releases[i].SourceIconURL = widget.Providers.AssetResolver("icons/" + string(releases[i].Source) + ".svg")
	}

	widget.Releases = releases
}

func (widget *Releases) Render() template.HTML {
	return widget.render(widget, assets.ReleasesTemplate)
}
