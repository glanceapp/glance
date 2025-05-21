package sources

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type githubIssuesSource struct {
	sourceBase    `yaml:",inline"`
	Issues        issueActivityList `yaml:"-"`
	Repositories  []*issueRequest   `yaml:"repositories"`
	Token         string            `yaml:"token"`
	Limit         int               `yaml:"limit"`
	CollapseAfter int               `yaml:"collapse-after"`
	ActivityTypes []string          `yaml:"activity-types"`
}

func (s *githubIssuesSource) Feed() []Activity {
	activities := make([]Activity, len(s.Issues))
	for i, issue := range s.Issues {
		activities[i] = issue
	}
	return activities
}

type issueActivity struct {
	ID            string
	Summary       string
	Description   string
	Source        string
	SourceIconURL string
	Repository    string
	IssueNumber   int
	title         string
	State         string
	ActivityType  string
	IssueType     string
	HTMLURL       string
	TimeUpdated   time.Time
	MatchScore    int
}

func (i issueActivity) UID() string {
	return i.ID
}

func (i issueActivity) Title() string {
	return i.title
}

func (i issueActivity) Body() string {
	return i.Description
}

func (i issueActivity) URL() string {
	return i.HTMLURL
}

func (i issueActivity) ImageURL() string {
	return i.SourceIconURL
}

func (i issueActivity) CreatedAt() time.Time {
	return i.TimeUpdated
}

type issueActivityList []issueActivity

func (i issueActivityList) sortByNewest() issueActivityList {
	sort.Slice(i, func(a, b int) bool {
		return i[a].TimeUpdated.After(i[b].TimeUpdated)
	})
	return i
}

type issueRequest struct {
	Repository string `yaml:"repository"`
	token      *string
}

func (i *issueRequest) UnmarshalYAML(node *yaml.Node) error {
	var repository string

	if err := node.Decode(&repository); err != nil {
		type issueRequestAlias issueRequest
		alias := (*issueRequestAlias)(i)
		if err := node.Decode(alias); err != nil {
			return fmt.Errorf("could not unmarshal repository into string or struct: %v", err)
		}
	}

	if i.Repository == "" {
		if repository == "" {
			return errors.New("repository is required")
		}
		i.Repository = repository
	}

	return nil
}

func (s *githubIssuesSource) initialize() error {
	s.withTitle("Issue Activity").withCacheDuration(30 * time.Minute)

	if s.Limit <= 0 {
		s.Limit = 10
	}

	if s.CollapseAfter == 0 || s.CollapseAfter < -1 {
		s.CollapseAfter = 5
	}

	if len(s.ActivityTypes) == 0 {
		s.ActivityTypes = []string{"opened", "closed", "commented"}
	}

	for i := range s.Repositories {
		r := s.Repositories[i]
		if s.Token != "" {
			r.token = &s.Token
		}
	}

	return nil
}

func (s *githubIssuesSource) update(ctx context.Context) {
	activities, err := fetchIssueActivities(s.Repositories, s.ActivityTypes)

	if !s.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(activities) > s.Limit {
		activities = activities[:s.Limit]
	}

	for i := range activities {
		activities[i].SourceIconURL = s.Providers.assetResolver("icons/github.svg")
	}

	s.Issues = activities
}

type githubIssueResponse struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	State       string    `json:"state"`
	HTMLURL     string    `json:"html_url"`
	UpdatedAt   string    `json:"updated_at"`
	Body        string    `json:"body"`
	PullRequest *struct{} `json:"pull_request,omitempty"`
}

type githubIssueCommentResponse struct {
	ID        int    `json:"ID"`
	Body      string `json:"body"`
	IssueURL  string `json:"issue_url"`
	HTMLURL   string `json:"html_url"`
	UpdatedAt string `json:"updated_at"`
}

func fetchIssueActivities(requests []*issueRequest, activityTypes []string) (issueActivityList, error) {
	job := newJob(fetchIssueActivityTask, requests).withWorkers(20)
	results, errs, err := workerPoolDo(job)
	if err != nil {
		return nil, err
	}

	var failed int
	activities := make(issueActivityList, 0, len(requests)*len(activityTypes))

	for i := range results {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to fetch issue activity", "repository", requests[i].Repository, "error", errs[i])
			continue
		}

		activities = append(activities, results[i]...)
	}

	if failed == len(requests) {
		return nil, errNoContent
	}

	activities.sortByNewest()

	if failed > 0 {
		return activities, fmt.Errorf("%w: could not get issue activities for %d repositories", errPartialContent, failed)
	}

	return activities, nil
}

func fetchIssueActivityTask(request *issueRequest) ([]issueActivity, error) {
	activities := make([]issueActivity, 0)

	issues, err := fetchLatestIssues(request)
	if err != nil {
		return nil, err
	}

	comments, err := fetchLatestComments(request)
	if err != nil {
		return nil, err
	}

	for _, issue := range issues {
		issueType := "issue"
		if issue.PullRequest != nil {
			issueType = "pull request"
		}
		activities = append(activities, issueActivity{
			ID:           fmt.Sprintf("issue-%d", issue.Number),
			Description:  issue.Body,
			Source:       "github",
			Repository:   request.Repository,
			IssueNumber:  issue.Number,
			title:        issue.Title,
			State:        issue.State,
			ActivityType: issue.State,
			IssueType:    issueType,
			HTMLURL:      issue.HTMLURL,
			TimeUpdated:  parseRFC3339Time(issue.UpdatedAt),
		})
	}

	for _, comment := range comments {
		issueNumber := 0
		if comment.IssueURL != "" {
			parts := strings.Split(comment.IssueURL, "/")
			if len(parts) > 0 {
				if n, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
					issueNumber = n
				}
			}
		}
		title := comment.Body
		titleLimit := 40
		if len(title) > titleLimit {
			title = title[:titleLimit] + "..."
		}
		activities = append(activities, issueActivity{
			ID:           fmt.Sprintf("comment-%d", comment.ID),
			Description:  comment.Body,
			IssueNumber:  issueNumber,
			Source:       "github",
			Repository:   request.Repository,
			ActivityType: "commented",
			title:        title,
			IssueType:    "issue",
			HTMLURL:      comment.HTMLURL,
			TimeUpdated:  parseRFC3339Time(comment.UpdatedAt),
		})
	}

	return activities, nil
}

func fetchLatestIssues(request *issueRequest) ([]githubIssueResponse, error) {
	httpRequest, err := http.NewRequest(
		"GET",
		fmt.Sprintf("https://api.github.com/repos/%s/issues?state=all&sort=updated&direction=desc&per_page=10", request.Repository),
		nil,
	)
	if err != nil {
		return nil, err
	}

	// TODO(pulse): Change secrets config approach
	if request.token != nil {
		httpRequest.Header.Add("Authorization", "Bearer "+(*request.token))
	}
	envToken := os.Getenv("GITHUB_TOKEN")
	if envToken != "" {
		httpRequest.Header.Add("Authorization", "Bearer "+envToken)
	}

	response, err := decodeJsonFromRequest[[]githubIssueResponse](defaultHTTPClient, httpRequest)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func fetchLatestComments(request *issueRequest) ([]githubIssueCommentResponse, error) {
	httpRequest, err := http.NewRequest(
		"GET",
		fmt.Sprintf("https://api.github.com/repos/%s/issues/comments?sort=updated&direction=desc&per_page=10", request.Repository),
		nil,
	)
	if err != nil {
		return nil, err
	}

	// TODO(pulse): Change secrets config approach
	if request.token != nil {
		httpRequest.Header.Add("Authorization", "Bearer "+(*request.token))
	}
	envToken := os.Getenv("GITHUB_TOKEN")
	if envToken != "" {
		httpRequest.Header.Add("Authorization", "Bearer "+envToken)
	}

	response, err := decodeJsonFromRequest[[]githubIssueCommentResponse](defaultHTTPClient, httpRequest)
	if err != nil {
		return nil, err
	}

	return response, nil
}
