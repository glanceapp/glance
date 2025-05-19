package glance

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var issuesWidgetTemplate = mustParseTemplate("issues.html", "widget-base.html")

type issuesWidget struct {
	widgetBase    `yaml:",inline"`
	Issues        issueActivityList `yaml:"-"`
	Repositories  []*issueRequest   `yaml:"repositories"`
	Token         string            `yaml:"token"`
	Limit         int               `yaml:"limit"`
	CollapseAfter int               `yaml:"collapse-after"`
	ActivityTypes []string          `yaml:"activity-types"`
}

type issueActivity struct {
	ID            string
	Summary       string
	Description   string
	Source        string
	SourceIconURL string
	Repository    string
	IssueNumber   int
	Title         string
	State         string
	ActivityType  string
	IssueType     string
	URL           string
	TimeUpdated   time.Time
	MatchScore    int
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

func (widget *issuesWidget) initialize() error {
	widget.withTitle("Issue Activity").withCacheDuration(30 * time.Minute)

	if widget.Limit <= 0 {
		widget.Limit = 10
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	if len(widget.ActivityTypes) == 0 {
		widget.ActivityTypes = []string{"opened", "closed", "commented"}
	}

	for i := range widget.Repositories {
		r := widget.Repositories[i]
		if widget.Token != "" {
			r.token = &widget.Token
		}
	}

	return nil
}

func (widget *issuesWidget) update(ctx context.Context) {
	activities, err := fetchIssueActivities(widget.Repositories, widget.ActivityTypes)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(activities) > widget.Limit {
		activities = activities[:widget.Limit]
	}

	for i := range activities {
		activities[i].SourceIconURL = widget.Providers.assetResolver("icons/github.svg")
	}

	widget.Issues = activities

	if widget.filterQuery != "" {
		widget.rankForRelevancy(widget.filterQuery)
	}
}

func (widget *issuesWidget) rankForRelevancy(query string) {
	llm, err := NewLLM()
	if err != nil {
		slog.Error("Failed to initialize LLM", "error", err)
		return
	}

	feed := make([]feedEntry, 0, len(widget.Issues))

	for _, e := range widget.Issues {
		feed = append(feed, feedEntry{
			ID:          e.ID,
			Title:       e.Title,
			Description: e.Description,
			URL:         e.URL,
			ImageURL:    e.SourceIconURL,
			PublishedAt: e.TimeUpdated,
		})
	}

	matches, err := llm.filterFeed(context.Background(), feed, query)
	if err != nil {
		slog.Error("Failed to filter issues", "error", err)
		return
	}

	matchesMap := make(map[string]feedMatch)
	for _, match := range matches {
		matchesMap[match.ID] = match
	}

	filtered := make([]issueActivity, 0, len(matches))
	for _, e := range widget.Issues {
		if match, ok := matchesMap[e.ID]; ok {
			e.Summary = match.Highlight
			e.MatchScore = match.Score
			filtered = append(filtered, e)
		}
	}

	widget.Issues = filtered
}

func (widget *issuesWidget) Render() template.HTML {
	return widget.renderTemplate(widget, issuesWidgetTemplate)
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
	ID        int    `json:"id"`
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
			Title:        issue.Title,
			State:        issue.State,
			ActivityType: issue.State,
			IssueType:    issueType,
			URL:          issue.HTMLURL,
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
			Title:        title,
			IssueType:    "issue",
			URL:          comment.HTMLURL,
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
