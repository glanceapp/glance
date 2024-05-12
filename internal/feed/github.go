package feed

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type githubReleaseResponseJson struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	HtmlUrl     string `json:"html_url"`
	Draft       bool   `json:"draft"`
	PreRelease  bool   `json:"prerelease"`
	Reactions   struct {
		Downvotes int `json:"-1"`
	} `json:"reactions"`
}

func parseGithubTime(t string) time.Time {
	parsedTime, err := time.Parse("2006-01-02T15:04:05Z", t)

	if err != nil {
		return time.Now()
	}

	return parsedTime
}

func FetchLatestReleasesFromGithub(repositories []string, token string) (AppReleases, error) {
	appReleases := make(AppReleases, 0, len(repositories))

	if len(repositories) == 0 {
		return appReleases, nil
	}

	requests := make([]*http.Request, len(repositories))

	for i, repository := range repositories {
		request, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=10", repository), nil)

		if token != "" {
			request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		}

		requests[i] = request
	}

	task := decodeJsonFromRequestTask[[]githubReleaseResponseJson](defaultClient)
	job := newJob(task, requests).withWorkers(15)
	responses, errs, err := workerPoolDo(job)

	if err != nil {
		return nil, err
	}

	var failed int

	for i := range responses {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to fetch or parse github release", "error", errs[i], "url", requests[i].URL)
			continue
		}

		releases := responses[i]

		if len(releases) < 1 {
			failed++
			slog.Error("No releases found", "repository", repositories[i], "url", requests[i].URL)
			continue
		}

		var liveRelease *githubReleaseResponseJson

		for i := range releases {
			release := &releases[i]

			if !release.Draft && !release.PreRelease {
				liveRelease = release
				break
			}
		}

		if liveRelease == nil {
			slog.Error("No live release found", "repository", repositories[i], "url", requests[i].URL)
			continue
		}

		version := liveRelease.TagName

		if version[0] != 'v' {
			version = "v" + version
		}

		appReleases = append(appReleases, AppRelease{
			Name:         repositories[i],
			Version:      version,
			NotesUrl:     liveRelease.HtmlUrl,
			TimeReleased: parseGithubTime(liveRelease.PublishedAt),
			Downvotes:    liveRelease.Reactions.Downvotes,
		})
	}

	if len(appReleases) == 0 {
		return nil, ErrNoContent
	}

	appReleases.SortByNewest()

	if failed > 0 {
		return appReleases, fmt.Errorf("%w: could not get %d releases", ErrPartialContent, failed)
	}

	return appReleases, nil
}

type GithubTicket struct {
	Number    int
	CreatedAt time.Time
	Title     string
}

type RepositoryDetails struct {
	Name             string
	Stars            int
	Forks            int
	OpenPullRequests int
	PullRequests     []GithubTicket
	OpenIssues       int
	Issues           []GithubTicket
}

type githubRepositoryDetailsResponseJson struct {
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

func FetchRepositoryDetailsFromGithub(repository string, token string, maxPRs int, maxIssues int) (RepositoryDetails, error) {
	repositoryRequest, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s", repository), nil)

	if err != nil {
		return RepositoryDetails{}, fmt.Errorf("%w: could not create request with repository: %v", ErrNoContent, err)
	}

	PRsRequest, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/search/issues?q=is:pr+is:open+repo:%s&per_page=%d", repository, maxPRs), nil)
	issuesRequest, _ := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/search/issues?q=is:issue+is:open+repo:%s&per_page=%d", repository, maxIssues), nil)

	if token != "" {
		token = fmt.Sprintf("Bearer %s", token)
		repositoryRequest.Header.Add("Authorization", token)
		PRsRequest.Header.Add("Authorization", token)
		issuesRequest.Header.Add("Authorization", token)
	}

	var detailsResponse githubRepositoryDetailsResponseJson
	var detailsErr error
	var PRsResponse githubTicketResponseJson
	var PRsErr error
	var issuesResponse githubTicketResponseJson
	var issuesErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go (func() {
		defer wg.Done()
		detailsResponse, detailsErr = decodeJsonFromRequest[githubRepositoryDetailsResponseJson](defaultClient, repositoryRequest)
	})()

	if maxPRs > 0 {
		wg.Add(1)
		go (func() {
			defer wg.Done()
			PRsResponse, PRsErr = decodeJsonFromRequest[githubTicketResponseJson](defaultClient, PRsRequest)
		})()
	}

	if maxIssues > 0 {
		wg.Add(1)
		go (func() {
			defer wg.Done()
			issuesResponse, issuesErr = decodeJsonFromRequest[githubTicketResponseJson](defaultClient, issuesRequest)
		})()
	}

	wg.Wait()

	if detailsErr != nil {
		return RepositoryDetails{}, fmt.Errorf("%w: could not get repository details: %s", ErrNoContent, detailsErr)
	}

	details := RepositoryDetails{
		Name:         detailsResponse.Name,
		Stars:        detailsResponse.Stars,
		Forks:        detailsResponse.Forks,
		PullRequests: make([]GithubTicket, 0, len(PRsResponse.Tickets)),
		Issues:       make([]GithubTicket, 0, len(issuesResponse.Tickets)),
	}

	err = nil

	if maxPRs > 0 {
		if PRsErr != nil {
			err = fmt.Errorf("%w: could not get PRs: %s", ErrPartialContent, PRsErr)
		} else {
			details.OpenPullRequests = PRsResponse.Count

			for i := range PRsResponse.Tickets {
				details.PullRequests = append(details.PullRequests, GithubTicket{
					Number:    PRsResponse.Tickets[i].Number,
					CreatedAt: parseGithubTime(PRsResponse.Tickets[i].CreatedAt),
					Title:     PRsResponse.Tickets[i].Title,
				})
			}
		}
	}

	if maxIssues > 0 {
		if issuesErr != nil {
			// TODO: fix, overwriting the previous error
			err = fmt.Errorf("%w: could not get issues: %s", ErrPartialContent, issuesErr)
		} else {
			details.OpenIssues = issuesResponse.Count

			for i := range issuesResponse.Tickets {
				details.Issues = append(details.Issues, GithubTicket{
					Number:    issuesResponse.Tickets[i].Number,
					CreatedAt: parseGithubTime(issuesResponse.Tickets[i].CreatedAt),
					Title:     issuesResponse.Tickets[i].Title,
				})
			}
		}
	}

	return details, err
}
