package glance

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var giteeRepoWidgetTemplate = mustParseTemplate("gitee-repository.html", "widget-base.html")

type giteeRepositoryWidget struct {
	widgetBase        `yaml:",inline"`
	RequestedRepo     string           `yaml:"repository"`
	Token             string           `yaml:"token"`
	PullRequestsLimit int              `yaml:"pull-requests-limit"`
	IssuesLimit       int              `yaml:"issues-limit"`
	CommitsLimit      int              `yaml:"commits-limit"`
	RepoInfo          giteeRepoDetails `yaml:"-"`
}

func (widget *giteeRepositoryWidget) initialize() error {
	widget.withTitle("Gitee Repository").withCacheDuration(1 * time.Hour)

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

func (widget *giteeRepositoryWidget) update(ctx context.Context) {
	details, err := fetchGiteeRepoDetails(
		widget.RequestedRepo,
		string(widget.Token),
		widget.PullRequestsLimit,
		widget.IssuesLimit,
		widget.CommitsLimit,
	)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.RepoInfo = details
}

func (widget *giteeRepositoryWidget) Render() template.HTML {
	return widget.renderTemplate(widget, giteeRepoWidgetTemplate)
}

type giteeRepoDetails struct {
	Name             string
	Stars            int
	Forks            int
	OpenPullRequests int
	PullRequests     []GiteePrTicket
	OpenIssues       int
	Issues           []GiteeTicket
	LastCommits      int
	Commits          []GiteeCommitInfo
}

type GiteeTicket struct {
	Number    string
	CreatedAt time.Time
	Title     string
}

type GiteePrTicket struct {
	Number    int
	CreatedAt time.Time
	Title     string
}

type GiteeCommitInfo struct {
	Sha       string
	Author    string
	CreatedAt time.Time
	Message   string
}

type GiteeRepoResponseJSON struct {
	Name  string `json:"full_name"`
	Stars int    `json:"stargazers_count"`
	Forks int    `json:"forks_count"`
}

type GiteeTicketResponseJSON []struct {
	Number    string `json:"number"`
	CreatedAt string `json:"created_at"`
	Title     string `json:"title"`
}

type GiteePrTicketResponseJSON []struct {
	Number    int    `json:"number"`
	CreatedAt string `json:"created_at"`
	Title     string `json:"title"`
}

type GiteeCommitResponseJSON struct {
	Sha    string `json:"sha"`
	Commit struct {
		Author struct {
			Name string `json:"name"`
			Date string `json:"date"`
		} `json:"author"`
		Message string `json:"message"`
	} `json:"commit"`
}

func getTotalCountFromHeader(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	totalCountStr := resp.Header.Get("total_count")
	totalCount, _ := strconv.Atoi(totalCountStr)
	return totalCount
}

func fetchGiteeRepoDetails(repo string, token string, maxPRs int, maxIssues int, maxCommits int) (giteeRepoDetails, error) {
	var repoURL, PRsURL, issuesURL, commitsURL string
	if token != "" {
		repoURL = fmt.Sprintf("https://gitee.com/api/v5/repos/%s?access_token=%s", repo, token)
		if maxPRs > 0 {
			PRsURL = fmt.Sprintf("https://gitee.com/api/v5/repos/%s/pulls?state=open&per_page=%d&access_token=%s", repo, maxPRs, token)
		}
		if maxIssues > 0 {
			issuesURL = fmt.Sprintf("https://gitee.com/api/v5/repos/%s/issues?state=open&page=1&per_page=%d&access_token=%s", repo, maxIssues, token)
		}
		if maxCommits > 0 {
			commitsURL = fmt.Sprintf("https://gitee.com/api/v5/repos/%s/commits?per_page=%d&access_token=%s", repo, maxCommits, token)
		}
	} else {
		repoURL = fmt.Sprintf("https://gitee.com/api/v5/repos/%s", repo)
		if maxPRs > 0 {
			PRsURL = fmt.Sprintf("https://gitee.com/api/v5/repos/%s/pulls?state=open&per_page=%d", repo, maxPRs)
		}
		if maxIssues > 0 {
			issuesURL = fmt.Sprintf("https://gitee.com/api/v5/repos/%s/issues?state=open&page=1&per_page=%d", repo, maxIssues)
		}
		if maxCommits > 0 {
			commitsURL = fmt.Sprintf("https://gitee.com/api/v5/repos/%s/commits?per_page=%d", repo, maxCommits)
		}
	}

	repoRequest, err := http.NewRequest("GET", repoURL, nil)
	if err != nil {
		return giteeRepoDetails{}, fmt.Errorf("%w: could not create request with repository: %v", errNoContent, err)
	}

	var PRsRequest, issuesRequest, CommitsRequest *http.Request
	if maxPRs > 0 {
		PRsRequest, err = http.NewRequest("GET", PRsURL, nil)
		if err != nil {
			return giteeRepoDetails{}, fmt.Errorf("%w: could not create request for PRs: %v", errNoContent, err)
		}
	}

	if maxIssues > 0 {
		issuesRequest, err = http.NewRequest("GET", issuesURL, nil)
		if err != nil {
			return giteeRepoDetails{}, fmt.Errorf("%w: could not create request for issues: %v", errNoContent, err)
		}
	}

	if maxCommits > 0 {
		CommitsRequest, err = http.NewRequest("GET", commitsURL, nil)
		if err != nil {
			return giteeRepoDetails{}, fmt.Errorf("%w: could not create request for commits: %v", errNoContent, err)
		}
	}

	var repoResponse GiteeRepoResponseJSON
	var detailsErr error
	var PRsResponse GiteePrTicketResponseJSON
	var PRsErr error
	var issuesResponse GiteeTicketResponseJSON
	var issuesErr error
	var commitsResponse []GiteeCommitResponseJSON
	var CommitsErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go (func() {
		defer wg.Done()
		repoResponse, detailsErr = decodeJsonFromRequest[GiteeRepoResponseJSON](defaultHTTPClient, repoRequest)
	})()

	if maxPRs > 0 {
		wg.Add(1)
		go (func() {
			defer wg.Done()
			PRsResponse, PRsErr = decodeJsonFromRequest[GiteePrTicketResponseJSON](defaultHTTPClient, PRsRequest)
		})()
	}

	if maxIssues > 0 {
		wg.Add(1)
		go (func() {
			defer wg.Done()
			issuesResponse, issuesErr = decodeJsonFromRequest[GiteeTicketResponseJSON](defaultHTTPClient, issuesRequest)
		})()
	}

	if maxCommits > 0 {
		wg.Add(1)
		go (func() {
			defer wg.Done()
			commitsResponse, CommitsErr = decodeJsonFromRequest[[]GiteeCommitResponseJSON](defaultHTTPClient, CommitsRequest)
		})()
	}

	wg.Wait()

	if detailsErr != nil {
		return giteeRepoDetails{}, fmt.Errorf("%w: could not get repository details: %s", errNoContent, detailsErr)
	}

	details := giteeRepoDetails{
		Name:         repoResponse.Name,
		Stars:        repoResponse.Stars,
		Forks:        repoResponse.Forks,
		PullRequests: make([]GiteePrTicket, 0, len(PRsResponse)),
		Issues:       make([]GiteeTicket, 0, len(issuesResponse)),
		Commits:      make([]GiteeCommitInfo, 0, len(commitsResponse)),
	}

	err = nil

	if maxPRs > 0 {
		if PRsErr != nil {
			err = fmt.Errorf("%w: could not get PRs: %s", errPartialContent, PRsErr)
		} else {
			details.OpenPullRequests = getTotalCountFromHeader(PRsRequest.Response)

			for i := range PRsResponse {
				details.PullRequests = append(details.PullRequests, GiteePrTicket{
					Number:    PRsResponse[i].Number,
					CreatedAt: parseRFC3339Time(PRsResponse[i].CreatedAt),
					Title:     PRsResponse[i].Title,
				})
			}
		}
	}

	if maxIssues > 0 {
		if issuesErr != nil {
			err = fmt.Errorf("%w: could not get issues: %s", errPartialContent, issuesErr)
		} else {
			details.OpenIssues = getTotalCountFromHeader(issuesRequest.Response)

			for i := range issuesResponse {
				details.Issues = append(details.Issues, GiteeTicket{
					Number:    issuesResponse[i].Number,
					CreatedAt: parseRFC3339Time(issuesResponse[i].CreatedAt),
					Title:     issuesResponse[i].Title,
				})
			}
		}
	}

	if maxCommits > 0 {
		if CommitsErr != nil {
			err = fmt.Errorf("%w: could not get commits: %s", errPartialContent, CommitsErr)
		} else {
			for i := range commitsResponse {
				details.Commits = append(details.Commits, GiteeCommitInfo{
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
