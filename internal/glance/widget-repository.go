package glance

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"
)

var repositoryWidgetTemplate = mustParseTemplate("repository.html", "widget-base.html")

type repositoryWidget struct {
	widgetBase          `yaml:",inline"`
	RequestedRepository string     `yaml:"repository"`
	Token               string     `yaml:"token"`
	PullRequestsLimit   int        `yaml:"pull-requests-limit"`
	IssuesLimit         int        `yaml:"issues-limit"`
	CommitsLimit        int        `yaml:"commits-limit"`
	Repository          repository `yaml:"-"`
}

func (widget *repositoryWidget) initialize() error {
	widget.withTitle("Repository").withCacheDuration(1 * time.Hour)

	if widget.PullRequestsLimit == 0 || widget.PullRequestsLimit < -1 {
		widget.PullRequestsLimit = 3
	}

	if widget.IssuesLimit == 0 || widget.IssuesLimit < -1 {
		widget.IssuesLimit = 3
	}

	if widget.CommitsLimit == 0 || widget.CommitsLimit < -1 {
		widget.CommitsLimit = -1
	}

	return nil
}

func (widget *repositoryWidget) update(ctx context.Context) {
	details, err := fetchRepositoryDetailsFromGithub(
		widget.RequestedRepository,
		string(widget.Token),
		widget.PullRequestsLimit,
		widget.IssuesLimit,
		widget.CommitsLimit,
	)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.Repository = details
}

func (widget *repositoryWidget) Render() template.HTML {
	return widget.renderTemplate(widget, repositoryWidgetTemplate)
}

type repository struct {
	Name             string
	Stars            int
	Forks            int
	OpenPullRequests int
	PullRequests     []githubTicket
	OpenIssues       int
	Issues           []githubTicket
	LastCommits      int
	Commits          []githubCommitDetails
}

type githubTicket struct {
	Number    int
	CreatedAt time.Time
	Title     string
}

type githubCommitDetails struct {
	Sha       string
	Author    string
	CreatedAt time.Time
	Message   string
}

type githubRepositoryResponseJson struct {
	Name  string `json:"full_name"`
	Stars int    `json:"stargazers_count"`
	Forks int    `json:"forks_count"`
}

type githubTicketResponseJson struct {
	Count   int `json:"total_count"`
	Tickets []struct {
		Number    int    `json:"number"`
		CreatedAt string `json:"created_at"`
		Title     string `json:"title"`
	} `json:"items"`
}

type gitHubCommitResponseJson struct {
	Sha    string `json:"sha"`
	Commit struct {
		Author struct {
			Name string `json:"name"`
			Date string `json:"date"`
		} `json:"author"`
		Message string `json:"message"`
	} `json:"commit"`
}

func fetchRepositoryDetailsFromGithub(repo string, token string, maxPRs int, maxIssues int, maxCommits int) (repository, error) {
	repositoryRequest, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s", repo), nil)
	if err != nil {
		return repository{}, fmt.Errorf("%w: could not create request with repository: %v", errNoContent, err)
	}

	PRsRequest, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/search/issues?q=is:pr+is:open+repo:%s&per_page=%d", repo, maxPRs), nil)
	issuesRequest, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/search/issues?q=is:issue+is:open+repo:%s&per_page=%d", repo, maxIssues), nil)
	CommitsRequest, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/commits?per_page=%d", repo, maxCommits), nil)

	if token != "" {
		token = fmt.Sprintf("Bearer %s", token)
		repositoryRequest.Header.Add("Authorization", token)
		PRsRequest.Header.Add("Authorization", token)
		issuesRequest.Header.Add("Authorization", token)
		CommitsRequest.Header.Add("Authorization", token)
	}

	var repositoryResponse githubRepositoryResponseJson
	var detailsErr error
	var PRsResponse githubTicketResponseJson
	var PRsErr error
	var issuesResponse githubTicketResponseJson
	var issuesErr error
	var commitsResponse []gitHubCommitResponseJson
	var CommitsErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go (func() {
		defer wg.Done()
		repositoryResponse, detailsErr = decodeJsonFromRequest[githubRepositoryResponseJson](defaultHTTPClient, repositoryRequest)
	})()

	if maxPRs > 0 {
		wg.Add(1)
		go (func() {
			defer wg.Done()
			PRsResponse, PRsErr = decodeJsonFromRequest[githubTicketResponseJson](defaultHTTPClient, PRsRequest)
		})()
	}

	if maxIssues > 0 {
		wg.Add(1)
		go (func() {
			defer wg.Done()
			issuesResponse, issuesErr = decodeJsonFromRequest[githubTicketResponseJson](defaultHTTPClient, issuesRequest)
		})()
	}

	if maxCommits > 0 {
		wg.Add(1)
		go (func() {
			defer wg.Done()
			commitsResponse, CommitsErr = decodeJsonFromRequest[[]gitHubCommitResponseJson](defaultHTTPClient, CommitsRequest)
		})()
	}

	wg.Wait()

	if detailsErr != nil {
		return repository{}, fmt.Errorf("%w: could not get repository details: %s", errNoContent, detailsErr)
	}

	details := repository{
		Name:         repositoryResponse.Name,
		Stars:        repositoryResponse.Stars,
		Forks:        repositoryResponse.Forks,
		PullRequests: make([]githubTicket, 0, len(PRsResponse.Tickets)),
		Issues:       make([]githubTicket, 0, len(issuesResponse.Tickets)),
		Commits:      make([]githubCommitDetails, 0, len(commitsResponse)),
	}

	err = nil

	if maxPRs > 0 {
		if PRsErr != nil {
			err = fmt.Errorf("%w: could not get PRs: %s", errPartialContent, PRsErr)
		} else {
			details.OpenPullRequests = PRsResponse.Count

			for i := range PRsResponse.Tickets {
				details.PullRequests = append(details.PullRequests, githubTicket{
					Number:    PRsResponse.Tickets[i].Number,
					CreatedAt: parseRFC3339Time(PRsResponse.Tickets[i].CreatedAt),
					Title:     PRsResponse.Tickets[i].Title,
				})
			}
		}
	}

	if maxIssues > 0 {
		if issuesErr != nil {
			// TODO: fix, overwriting the previous error
			err = fmt.Errorf("%w: could not get issues: %s", errPartialContent, issuesErr)
		} else {
			details.OpenIssues = issuesResponse.Count

			for i := range issuesResponse.Tickets {
				details.Issues = append(details.Issues, githubTicket{
					Number:    issuesResponse.Tickets[i].Number,
					CreatedAt: parseRFC3339Time(issuesResponse.Tickets[i].CreatedAt),
					Title:     issuesResponse.Tickets[i].Title,
				})
			}
		}
	}

	if maxCommits > 0 {
		if CommitsErr != nil {
			err = fmt.Errorf("%w: could not get commits: %s", errPartialContent, CommitsErr)
		} else {
			for i := range commitsResponse {
				details.Commits = append(details.Commits, githubCommitDetails{
					Sha:       commitsResponse[i].Sha,
					Author:    commitsResponse[i].Commit.Author.Name,
					CreatedAt: parseRFC3339Time(commitsResponse[i].Commit.Author.Date),
					Message:   strings.SplitN(commitsResponse[i].Commit.Message, "\n\n", 2)[0],
				})
			}
		}
	}

	return details, err
}
