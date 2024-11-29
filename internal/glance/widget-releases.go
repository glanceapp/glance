package glance

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var releasesWidgetTemplate = mustParseTemplate("releases.html", "widget-base.html")

type releasesWidget struct {
	widgetBase      `yaml:",inline"`
	Releases        appReleaseList    `yaml:"-"`
	releaseRequests []*releaseRequest `yaml:"-"`
	Repositories    []string          `yaml:"repositories"`
	Token           optionalEnvField  `yaml:"token"`
	GitLabToken     optionalEnvField  `yaml:"gitlab-token"`
	Limit           int               `yaml:"limit"`
	CollapseAfter   int               `yaml:"collapse-after"`
	ShowSourceIcon  bool              `yaml:"show-source-icon"`
}

func (widget *releasesWidget) initialize() error {
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
		var request *releaseRequest
		if len(parts) == 1 {
			request = &releaseRequest{
				source:     releaseSourceGithub,
				repository: repository,
			}

			if widget.Token != "" {
				request.token = &tokenAsString
			}
		} else if len(parts) == 2 {
			if parts[0] == string(releaseSourceGitlab) {
				request = &releaseRequest{
					source:     releaseSourceGitlab,
					repository: parts[1],
				}

				if widget.GitLabToken != "" {
					request.token = &gitLabTokenAsString
				}
			} else if parts[0] == string(releaseSourceDockerHub) {
				request = &releaseRequest{
					source:     releaseSourceDockerHub,
					repository: parts[1],
				}
			} else if parts[0] == string(releaseSourceCodeberg) {
				request = &releaseRequest{
					source:     releaseSourceCodeberg,
					repository: parts[1],
				}
			} else {
				return errors.New("invalid repository source " + parts[0])
			}
		}

		widget.releaseRequests = append(widget.releaseRequests, request)
	}

	return nil
}

func (widget *releasesWidget) update(ctx context.Context) {
	releases, err := fetchLatestReleases(widget.releaseRequests)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(releases) > widget.Limit {
		releases = releases[:widget.Limit]
	}

	for i := range releases {
		releases[i].SourceIconURL = widget.Providers.assetResolver("icons/" + string(releases[i].Source) + ".svg")
	}

	widget.Releases = releases
}

func (widget *releasesWidget) Render() template.HTML {
	return widget.renderTemplate(widget, releasesWidgetTemplate)
}

type releaseSource string

const (
	releaseSourceCodeberg  releaseSource = "codeberg"
	releaseSourceGithub    releaseSource = "github"
	releaseSourceGitlab    releaseSource = "gitlab"
	releaseSourceDockerHub releaseSource = "dockerhub"
)

type appRelease struct {
	Source        releaseSource
	SourceIconURL string
	Name          string
	Version       string
	NotesUrl      string
	TimeReleased  time.Time
	Downvotes     int
}

type appReleaseList []appRelease

func (r appReleaseList) sortByNewest() appReleaseList {
	sort.Slice(r, func(i, j int) bool {
		return r[i].TimeReleased.After(r[j].TimeReleased)
	})

	return r
}

type releaseRequest struct {
	source     releaseSource
	repository string
	token      *string
}

func fetchLatestReleases(requests []*releaseRequest) (appReleaseList, error) {
	job := newJob(fetchLatestReleaseTask, requests).withWorkers(20)
	results, errs, err := workerPoolDo(job)

	if err != nil {
		return nil, err
	}

	var failed int

	releases := make(appReleaseList, 0, len(requests))

	for i := range results {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to fetch release", "source", requests[i].source, "repository", requests[i].repository, "error", errs[i])
			continue
		}

		releases = append(releases, *results[i])
	}

	if failed == len(requests) {
		return nil, errNoContent
	}

	releases.sortByNewest()

	if failed > 0 {
		return releases, fmt.Errorf("%w: could not get %d releases", errPartialContent, failed)
	}

	return releases, nil
}

func fetchLatestReleaseTask(request *releaseRequest) (*appRelease, error) {
	switch request.source {
	case releaseSourceCodeberg:
		return fetchLatestCodebergRelease(request)
	case releaseSourceGithub:
		return fetchLatestGithubRelease(request)
	case releaseSourceGitlab:
		return fetchLatestGitLabRelease(request)
	case releaseSourceDockerHub:
		return fetchLatestDockerHubRelease(request)
	}

	return nil, errors.New("unsupported source")
}

type githubReleaseLatestResponseJson struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	HtmlUrl     string `json:"html_url"`
	Reactions   struct {
		Downvotes int `json:"-1"`
	} `json:"reactions"`
}

func fetchLatestGithubRelease(request *releaseRequest) (*appRelease, error) {
	httpRequest, err := http.NewRequest(
		"GET",
		fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", request.repository),
		nil,
	)

	if err != nil {
		return nil, err
	}

	if request.token != nil {
		httpRequest.Header.Add("Authorization", "Bearer "+(*request.token))
	}

	response, err := decodeJsonFromRequest[githubReleaseLatestResponseJson](defaultClient, httpRequest)

	if err != nil {
		return nil, err
	}

	return &appRelease{
		Source:       releaseSourceGithub,
		Name:         request.repository,
		Version:      normalizeVersionFormat(response.TagName),
		NotesUrl:     response.HtmlUrl,
		TimeReleased: parseRFC3339Time(response.PublishedAt),
		Downvotes:    response.Reactions.Downvotes,
	}, nil
}

type dockerHubRepositoryTagsResponse struct {
	Results []dockerHubRepositoryTagResponse `json:"results"`
}

type dockerHubRepositoryTagResponse struct {
	Name       string `json:"name"`
	LastPushed string `json:"tag_last_pushed"`
}

const dockerHubOfficialRepoTagURLFormat = "https://hub.docker.com/_/%s/tags?name=%s"
const dockerHubRepoTagURLFormat = "https://hub.docker.com/r/%s/tags?name=%s"
const dockerHubTagsURLFormat = "https://hub.docker.com/v2/namespaces/%s/repositories/%s/tags"
const dockerHubSpecificTagURLFormat = "https://hub.docker.com/v2/namespaces/%s/repositories/%s/tags/%s"

func fetchLatestDockerHubRelease(request *releaseRequest) (*appRelease, error) {

	nameParts := strings.Split(request.repository, "/")

	if len(nameParts) > 2 {
		return nil, fmt.Errorf("invalid repository name: %s", request.repository)
	} else if len(nameParts) == 1 {
		nameParts = []string{"library", nameParts[0]}
	}

	tagParts := strings.SplitN(nameParts[1], ":", 2)

	var requestURL string

	if len(tagParts) == 2 {
		requestURL = fmt.Sprintf(dockerHubSpecificTagURLFormat, nameParts[0], tagParts[0], tagParts[1])
	} else {
		requestURL = fmt.Sprintf(dockerHubTagsURLFormat, nameParts[0], nameParts[1])
	}

	httpRequest, err := http.NewRequest("GET", requestURL, nil)

	if err != nil {
		return nil, err
	}

	if request.token != nil {
		httpRequest.Header.Add("Authorization", "Bearer "+(*request.token))
	}

	var tag *dockerHubRepositoryTagResponse

	if len(tagParts) == 1 {
		response, err := decodeJsonFromRequest[dockerHubRepositoryTagsResponse](defaultClient, httpRequest)

		if err != nil {
			return nil, err
		}

		if len(response.Results) == 0 {
			return nil, fmt.Errorf("no tags found for repository: %s", request.repository)
		}

		tag = &response.Results[0]
	} else {
		response, err := decodeJsonFromRequest[dockerHubRepositoryTagResponse](defaultClient, httpRequest)

		if err != nil {
			return nil, err
		}

		tag = &response
	}

	var repo string
	var displayName string
	var notesURL string

	if len(tagParts) == 1 {
		repo = nameParts[1]
	} else {
		repo = tagParts[0]
	}

	if nameParts[0] == "library" {
		displayName = repo
		notesURL = fmt.Sprintf(dockerHubOfficialRepoTagURLFormat, repo, tag.Name)
	} else {
		displayName = nameParts[0] + "/" + repo
		notesURL = fmt.Sprintf(dockerHubRepoTagURLFormat, displayName, tag.Name)
	}

	return &appRelease{
		Source:       releaseSourceDockerHub,
		NotesUrl:     notesURL,
		Name:         displayName,
		Version:      tag.Name,
		TimeReleased: parseRFC3339Time(tag.LastPushed),
	}, nil
}

type gitlabReleaseResponseJson struct {
	TagName    string `json:"tag_name"`
	ReleasedAt string `json:"released_at"`
	Links      struct {
		Self string `json:"self"`
	} `json:"_links"`
}

func fetchLatestGitLabRelease(request *releaseRequest) (*appRelease, error) {
	httpRequest, err := http.NewRequest(
		"GET",
		fmt.Sprintf(
			"https://gitlab.com/api/v4/projects/%s/releases/permalink/latest",
			url.QueryEscape(request.repository),
		),
		nil,
	)

	if err != nil {
		return nil, err
	}

	if request.token != nil {
		httpRequest.Header.Add("PRIVATE-TOKEN", *request.token)
	}

	response, err := decodeJsonFromRequest[gitlabReleaseResponseJson](defaultClient, httpRequest)

	if err != nil {
		return nil, err
	}

	return &appRelease{
		Source:       releaseSourceGitlab,
		Name:         request.repository,
		Version:      normalizeVersionFormat(response.TagName),
		NotesUrl:     response.Links.Self,
		TimeReleased: parseRFC3339Time(response.ReleasedAt),
	}, nil
}

type codebergReleaseResponseJson struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	HtmlUrl     string `json:"html_url"`
}

func fetchLatestCodebergRelease(request *releaseRequest) (*appRelease, error) {
	httpRequest, err := http.NewRequest(
		"GET",
		fmt.Sprintf(
			"https://codeberg.org/api/v1/repos/%s/releases/latest",
			request.repository,
		),
		nil,
	)
	if err != nil {
		return nil, err
	}

	response, err := decodeJsonFromRequest[codebergReleaseResponseJson](defaultClient, httpRequest)

	if err != nil {
		return nil, err
	}
	return &appRelease{
		Source:       releaseSourceCodeberg,
		Name:         request.repository,
		Version:      normalizeVersionFormat(response.TagName),
		NotesUrl:     response.HtmlUrl,
		TimeReleased: parseRFC3339Time(response.PublishedAt),
	}, nil
}
