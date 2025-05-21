package sources

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

type hackerNewsSource struct {
	sourceBase          `yaml:",inline"`
	Posts               forumPostList `yaml:"-"`
	Limit               int           `yaml:"limit"`
	SortBy              string        `yaml:"sort-by"`
	ExtraSortBy         string        `yaml:"extra-sort-by"`
	CollapseAfter       int           `yaml:"collapse-after"`
	CommentsUrlTemplate string        `yaml:"comments-url-template"`
	ShowThumbnails      bool          `yaml:"-"`
}

func (s *hackerNewsSource) initialize() error {
	s.
		withTitle("Hacker News").
		withTitleURL("https://news.ycombinator.com/").
		withCacheDuration(30 * time.Minute)

	if s.Limit <= 0 {
		s.Limit = 15
	}

	if s.CollapseAfter == 0 || s.CollapseAfter < -1 {
		s.CollapseAfter = 5
	}

	if s.SortBy != "top" && s.SortBy != "new" && s.SortBy != "best" {
		s.SortBy = "top"
	}

	return nil
}

func (s *hackerNewsSource) update(ctx context.Context) {
	posts, err := fetchHackerNewsPosts(s.SortBy, 40, s.CommentsUrlTemplate)

	if !s.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if s.ExtraSortBy == "engagement" {
		posts.calculateEngagement()
		posts.sortByEngagement()
	}

	if s.Limit < len(posts) {
		posts = posts[:s.Limit]
	}

	s.Posts = posts
}

type hackerNewsPostResponseJson struct {
	Id           int    `json:"id"`
	Score        int    `json:"score"`
	Title        string `json:"title"`
	TargetUrl    string `json:"url,omitempty"`
	CommentCount int    `json:"descendants"`
	TimePosted   int64  `json:"time"`
}

func fetchHackerNewsPostIds(sort string) ([]int, error) {
	request, _ := http.NewRequest("GET", fmt.Sprintf("https://hacker-news.firebaseio.com/v0/%sstories.json", sort), nil)
	response, err := decodeJsonFromRequest[[]int](defaultHTTPClient, request)
	if err != nil {
		return nil, fmt.Errorf("%w: could not fetch list of post IDs", errNoContent)
	}

	return response, nil
}

func fetchHackerNewsPostsFromIds(postIds []int, commentsUrlTemplate string) (forumPostList, error) {
	requests := make([]*http.Request, len(postIds))

	for i, id := range postIds {
		request, _ := http.NewRequest("GET", fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", id), nil)
		requests[i] = request
	}

	task := decodeJsonFromRequestTask[hackerNewsPostResponseJson](defaultHTTPClient)
	job := newJob(task, requests).withWorkers(30)
	results, errs, err := workerPoolDo(job)
	if err != nil {
		return nil, err
	}

	posts := make(forumPostList, 0, len(postIds))

	for i, res := range results {
		if errs[i] != nil {
			slog.Error("Failed to fetch or parse hacker news post", "error", errs[i], "url", requests[i].URL)
			continue
		}

		var commentsUrl string

		if commentsUrlTemplate == "" {
			commentsUrl = "https://news.ycombinator.com/item?id=" + strconv.Itoa(res.Id)
		} else {
			commentsUrl = strings.ReplaceAll(commentsUrlTemplate, "{POST-ID}", strconv.Itoa(res.Id))
		}

		forumPost := forumPost{
			ID:              strconv.Itoa(res.Id),
			Title:           res.Title,
			Description:     res.Title,
			DiscussionUrl:   commentsUrl,
			TargetUrl:       res.TargetUrl,
			TargetUrlDomain: extractDomainFromUrl(res.TargetUrl),
			CommentCount:    res.CommentCount,
			Score:           res.Score,
			TimePosted:      time.Unix(res.TimePosted, 0),
		}

		article, err := readability.FromURL(forumPost.TargetUrl, 5*time.Second)
		if err == nil {
			forumPost.Description = article.TextContent
		} else {
			slog.Error("Failed to fetch hacker news article", "error", err, "url", forumPost.TargetUrl)
		}

		posts = append(posts, forumPost)
	}

	if len(posts) == 0 {
		return nil, errNoContent
	}

	if len(posts) != len(postIds) {
		return posts, fmt.Errorf("%w could not fetch some hacker news posts", errPartialContent)
	}

	return posts, nil
}

func fetchHackerNewsPosts(sort string, limit int, commentsUrlTemplate string) (forumPostList, error) {
	postIds, err := fetchHackerNewsPostIds(sort)
	if err != nil {
		return nil, err
	}

	if len(postIds) > limit {
		postIds = postIds[:limit]
	}

	return fetchHackerNewsPostsFromIds(postIds, commentsUrlTemplate)
}
