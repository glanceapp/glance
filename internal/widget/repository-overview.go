package widget

import (
	"context"
	"html/template"
	"time"

	"github.com/glanceapp/glance/internal/assets"
	"github.com/glanceapp/glance/internal/feed"
)

type RepositoryOverview struct {
	widgetBase          `yaml:",inline"`
	RequestedRepository string            `yaml:"repository"`
	Token               OptionalEnvString `yaml:"token"`
	PullRequestsLimit   int               `yaml:"pull-requests-limit"`
	IssuesLimit         int               `yaml:"issues-limit"`
	RepositoryDetails   feed.RepositoryDetails
}

func (widget *RepositoryOverview) Initialize() error {
	widget.withTitle("Repository").withCacheDuration(1 * time.Hour)

	if widget.PullRequestsLimit == 0 || widget.PullRequestsLimit < -1 {
		widget.PullRequestsLimit = 3
	}

	if widget.IssuesLimit == 0 || widget.IssuesLimit < -1 {
		widget.IssuesLimit = 3
	}

	return nil
}

func (widget *RepositoryOverview) Update(ctx context.Context) {
	details, err := feed.FetchRepositoryDetailsFromGithub(
		widget.RequestedRepository,
		string(widget.Token),
		widget.PullRequestsLimit,
		widget.IssuesLimit,
	)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.RepositoryDetails = details
}

func (widget *RepositoryOverview) Render() template.HTML {
	return widget.render(widget, assets.RepositoryOverviewTemplate)
}
