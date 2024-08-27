package feed

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type githubReleaseLatestResponseJson struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	HtmlUrl     string `json:"html_url"`
	Reactions   struct {
		Downvotes int `json:"-1"`
	} `json:"reactions"`
}

func fetchLatestGithubRelease(request *ReleaseRequest) (*AppRelease, error) {
	httpRequest, err := http.NewRequest(
		"GET",
		fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", request.Repository),
		nil,
	)

	if err != nil {
		return nil, err
	}

	if request.Token != nil {
		httpRequest.Header.Add("Authorization", "Bearer "+(*request.Token))
	}

	response, err := decodeJsonFromRequest[githubReleaseLatestResponseJson](defaultClient, httpRequest)

	if err != nil {
		return nil, err
	}

	version := response.TagName

	if len(version) > 0 && version[0] != 'v' {
		version = "v" + version
	}

	return &AppRelease{
		Source:       ReleaseSourceGithub,
		Name:         request.Repository,
		Version:      version,
		NotesUrl:     response.HtmlUrl,
		TimeReleased: parseRFC3339Time(response.PublishedAt),
		Downvotes:    response.Reactions.Downvotes,
	}, nil
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
					CreatedAt: parseRFC3339Time(PRsResponse.Tickets[i].CreatedAt),
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
					CreatedAt: parseRFC3339Time(issuesResponse.Tickets[i].CreatedAt),
					Title:     issuesResponse.Tickets[i].Title,
				})
			}
		}
	}

	return details, err
}
