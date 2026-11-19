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

	"gopkg.in/yaml.v3"
)

var releasesWidgetTemplate = mustParseTemplate("releases.html", "widget-base.html")

type releasesWidget struct {
	widgetBase     `yaml:",inline"`
	Releases       appReleaseList    `yaml:"-"`
	Repositories   []*releaseRequest `yaml:"repositories"`
	Token          string            `yaml:"token"`
	GitLabToken    string            `yaml:"gitlab-token"`
	Limit          int               `yaml:"limit"`
	CollapseAfter  int               `yaml:"collapse-after"`
	ShowSourceIcon bool              `yaml:"show-source-icon"`
}

func (widget *releasesWidget) initialize() error {
	widget.withTitle("Releases").withCacheDuration(2 * time.Hour)

	if widget.Limit <= 0 {
		widget.Limit = 10
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	for i := range widget.Repositories {
		r := widget.Repositories[i]

		if r.source == releaseSourceGithub && widget.Token != "" {
			r.token = &widget.Token
		} else if r.source == releaseSourceGitlab && widget.GitLabToken != "" {
			r.token = &widget.GitLabToken
		}
	}

	return nil
}

func (widget *releasesWidget) update(ctx context.Context) {
	releases, err := fetchLatestReleases(widget.Repositories)

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
	IncludePreleases bool   `yaml:"include-prereleases"`
	UseTags          bool   `yaml:"use-tags"`
	Repository       string `yaml:"repository"`

	source releaseSource
	token  *string
}

func (r *releaseRequest) UnmarshalYAML(node *yaml.Node) error {
	type releaseRequestAlias releaseRequest
	alias := (*releaseRequestAlias)(r)
	var repository string

	if err := node.Decode(&repository); err != nil {
		if err := node.Decode(alias); err != nil {
			return fmt.Errorf("could not umarshal repository into string or struct: %v", err)
		}
	}

	if r.Repository == "" {
		if repository == "" {
			return errors.New("repository is required")
		} else {
			r.Repository = repository
		}
	}

	parts := strings.SplitN(r.Repository, ":", 2)
	if len(parts) == 1 {
		r.source = releaseSourceGithub
	} else if len(parts) == 2 {
		r.Repository = parts[1]

		switch parts[0] {
		case string(releaseSourceGithub):
			r.source = releaseSourceGithub
		case string(releaseSourceGitlab):
			r.source = releaseSourceGitlab
		case string(releaseSourceDockerHub):
			r.source = releaseSourceDockerHub
		case string(releaseSourceCodeberg):
			r.source = releaseSourceCodeberg
		default:
			return errors.New("invalid source")
		}
	}

	if r.UseTags && r.source != releaseSourceGithub && r.source != releaseSourceGitlab {
		return fmt.Errorf("use-tags is only supported for GitHub and GitLab repositories, not %s", r.source)
	}

	return nil
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
			slog.Error("Failed to fetch release", "source", requests[i].source, "repository", requests[i].Repository, "error", errs[i])
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
		if request.UseTags {
			return fetchLatestGithubTag(request)
		}
		return fetchLatestGithubRelease(request)
	case releaseSourceGitlab:
		if request.UseTags {
			return fetchLatestGitLabTag(request)
		}
		return fetchLatestGitLabRelease(request)
	case releaseSourceDockerHub:
		return fetchLatestDockerHubRelease(request)
	}

	return nil, errors.New("unsupported source")
}

type githubReleaseResponseJson struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	HtmlUrl     string `json:"html_url"`
	Reactions   struct {
		Downvotes int `json:"-1"`
	} `json:"reactions"`
}

func fetchLatestGithubRelease(request *releaseRequest) (*appRelease, error) {
	var requestURL string
	if !request.IncludePreleases {
		requestURL = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", request.Repository)
	} else {
		requestURL = fmt.Sprintf("https://api.github.com/repos/%s/releases", request.Repository)
	}

	httpRequest, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}

	if request.token != nil {
		httpRequest.Header.Add("Authorization", "Bearer "+(*request.token))
	}

	var response githubReleaseResponseJson

	if !request.IncludePreleases {
		response, err = decodeJsonFromRequest[githubReleaseResponseJson](defaultHTTPClient, httpRequest)
		if err != nil {
			return nil, err
		}
	} else {
		responses, err := decodeJsonFromRequest[[]githubReleaseResponseJson](defaultHTTPClient, httpRequest)
		if err != nil {
			return nil, err
		}

		if len(responses) == 0 {
			return nil, fmt.Errorf("no releases found for repository %s", request.Repository)
		}

		response = responses[0]
	}

	return &appRelease{
		Source:       releaseSourceGithub,
		Name:         request.Repository,
		Version:      normalizeVersionFormat(response.TagName),
		NotesUrl:     response.HtmlUrl,
		TimeReleased: parseRFC3339Time(response.PublishedAt),
		Downvotes:    response.Reactions.Downvotes,
	}, nil
}

type githubTagsFeed struct {
	Entries []struct {
		Title   string `xml:"title"`
		Updated string `xml:"updated"`
		Link    struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
	} `xml:"entry"`
}

// GitHub's REST API returns tags in no particular date order and without dates,
// whereas the tags feed is sorted by date and doesn't count towards API rate limits.
func fetchLatestGithubTag(request *releaseRequest) (*appRelease, error) {
	httpRequest, err := http.NewRequest("GET", fmt.Sprintf("https://github.com/%s/tags.atom", request.Repository), nil)
	if err != nil {
		return nil, err
	}

	feed, err := decodeXmlFromRequest[githubTagsFeed](defaultHTTPClient, httpRequest)
	if err != nil {
		return nil, err
	}

	return githubTagToRelease(request.Repository, feed)
}

func githubTagToRelease(repository string, feed githubTagsFeed) (*appRelease, error) {
	if len(feed.Entries) == 0 {
		return nil, fmt.Errorf("no tags found for repository %s", repository)
	}

	entry := feed.Entries[0]

	return &appRelease{
		Source:       releaseSourceGithub,
		Name:         repository,
		Version:      normalizeVersionFormat(entry.Title),
		NotesUrl:     entry.Link.Href,
		TimeReleased: parseRFC3339Time(entry.Updated),
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
	nameParts := strings.Split(request.Repository, "/")

	if len(nameParts) > 2 {
		return nil, fmt.Errorf("invalid repository name: %s", request.Repository)
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
		response, err := decodeJsonFromRequest[dockerHubRepositoryTagsResponse](defaultHTTPClient, httpRequest)
		if err != nil {
			return nil, err
		}

		if len(response.Results) == 0 {
			return nil, fmt.Errorf("no tags found for repository: %s", request.Repository)
		}

		tag = &response.Results[0]
	} else {
		response, err := decodeJsonFromRequest[dockerHubRepositoryTagResponse](defaultHTTPClient, httpRequest)
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
			url.QueryEscape(request.Repository),
		),
		nil,
	)
	if err != nil {
		return nil, err
	}

	if request.token != nil {
		httpRequest.Header.Add("PRIVATE-TOKEN", *request.token)
	}

	response, err := decodeJsonFromRequest[gitlabReleaseResponseJson](defaultHTTPClient, httpRequest)
	if err != nil {
		return nil, err
	}

	return &appRelease{
		Source:       releaseSourceGitlab,
		Name:         request.Repository,
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
			request.Repository,
		),
		nil,
	)
	if err != nil {
		return nil, err
	}

	response, err := decodeJsonFromRequest[codebergReleaseResponseJson](defaultHTTPClient, httpRequest)
	if err != nil {
		return nil, err
	}

	return &appRelease{
		Source:       releaseSourceCodeberg,
		Name:         request.Repository,
		Version:      normalizeVersionFormat(response.TagName),
		NotesUrl:     response.HtmlUrl,
		TimeReleased: parseRFC3339Time(response.PublishedAt),
	}, nil
}

type gitlabTagResponseJson struct {
	Name   string `json:"name"`
	Commit struct {
		CreatedAt string `json:"created_at"`
	} `json:"commit"`
}

func fetchLatestGitLabTag(request *releaseRequest) (*appRelease, error) {
	httpRequest, err := http.NewRequest(
		"GET",
		fmt.Sprintf(
			"https://gitlab.com/api/v4/projects/%s/repository/tags?order_by=updated&sort=desc&per_page=1",
			url.QueryEscape(request.Repository),
		),
		nil,
	)
	if err != nil {
		return nil, err
	}

	if request.token != nil {
		httpRequest.Header.Add("PRIVATE-TOKEN", *request.token)
	}

	response, err := decodeJsonFromRequest[[]gitlabTagResponseJson](defaultHTTPClient, httpRequest)
	if err != nil {
		return nil, err
	}

	return gitlabTagToRelease(request.Repository, response)
}

func gitlabTagToRelease(repository string, tags []gitlabTagResponseJson) (*appRelease, error) {
	if len(tags) == 0 {
		return nil, fmt.Errorf("no tags found for repository %s", repository)
	}

	tag := tags[0]

	return &appRelease{
		Source:       releaseSourceGitlab,
		Name:         repository,
		Version:      normalizeVersionFormat(tag.Name),
		NotesUrl:     fmt.Sprintf("https://gitlab.com/%s/-/tags/%s", repository, url.PathEscape(tag.Name)),
		TimeReleased: parseRFC3339Time(tag.Commit.CreatedAt),
	}, nil
}
