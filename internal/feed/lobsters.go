package feed

import (
	"context"
	"fmt"
	"github.com/mmcdole/gofeed"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type lobstersPostResponseJson struct {
	ShortID          string   `json:"short_id"`
	ShortIDURL       string   `json:"short_id_url"`
	CreatedAt        string   `json:"created_at"`
	Title            string   `json:"title"`
	URL              string   `json:"url"`
	Score            int      `json:"score"`
	Flags            int      `json:"flags"`
	CommentCount     int      `json:"comment_count"`
	Description      string   `json:"description"`
	DescriptionPlain string   `json:"description_plain"`
	CommentsURL      string   `json:"comments_url"`
	SubmitterUser    string   `json:"submitter_user"`
	UserIsAuthor     bool     `json:"user_is_author"`
	Tags             []string `json:"tags"`
	Comments         []struct {
		ShortID        string `json:"short_id"`
		ShortIDURL     string `json:"short_id_url"`
		CreatedAt      string `json:"created_at"`
		UpdatedAt      string `json:"updated_at"`
		IsDeleted      bool   `json:"is_deleted"`
		IsModerated    bool   `json:"is_moderated"`
		Score          int    `json:"score"`
		Flags          int    `json:"flags"`
		ParentComment  any    `json:"parent_comment"`
		Comment        string `json:"comment"`
		CommentPlain   string `json:"comment_plain"`
		URL            string `json:"url"`
		Depth          int    `json:"depth"`
		CommentingUser string `json:"commenting_user"`
	} `json:"comments"`
}

var lobstersParser = gofeed.NewParser()

func getLobstersTopPostIds(feed string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	request := &RSSFeedRequest{
		Url:   feed,
		Title: "Lobsters",
	}

	f, err := lobstersParser.ParseURLWithContext(request.Url, ctx)

	if err != nil {
		return nil, fmt.Errorf("%w: could not fetch posts from %s", ErrNoContent, feed)
	}

	postIds := make([]string, 0, len(f.Items))

	for i := range f.Items {
		postIds = append(postIds, f.Items[i].GUID)
	}

	return postIds, nil
}

func getLobstersPostsFromIds(postIds []string) (ForumPosts, error) {
	requests := make([]*http.Request, len(postIds))

	for i, id := range postIds {
		request, _ := http.NewRequest("GET", id+".json", nil)
		requests[i] = request
	}

	task := decodeJsonFromRequestTask[lobstersPostResponseJson](defaultClient)
	job := newJob(task, requests).withWorkers(30)
	results, errs, err := workerPoolDo(job)

	if err != nil {
		return nil, err
	}

	posts := make(ForumPosts, 0, len(postIds))

	for i := range results {
		if errs[i] != nil {
			slog.Error("Failed to fetch or parse lobsters post", "error", errs[i], "url", requests[i].URL)
			continue
		}

		tags := strings.Join(results[i].Tags, ",")

		if tags != "" {
			tags = " [" + tags + "]"
		}

		createdAt, _ := time.Parse(time.RFC3339, results[i].CreatedAt)

		posts = append(posts, ForumPost{
			Title:           results[i].Title + tags,
			DiscussionUrl:   results[i].CommentsURL,
			TargetUrl:       results[i].URL,
			TargetUrlDomain: extractDomainFromUrl(results[i].URL),
			CommentCount:    results[i].CommentCount,
			Score:           results[i].Score,
			TimePosted:      createdAt,
		})
	}

	if len(posts) == 0 {
		return nil, ErrNoContent
	}

	if len(posts) != len(postIds) {
		return posts, fmt.Errorf("%w could not fetch some lobsters posts", ErrPartialContent)
	}

	return posts, nil
}

func FetchLobstersTopPosts(feed string, limit int) (ForumPosts, error) {
	postIds, err := getLobstersTopPostIds(feed)

	if err != nil {
		return nil, err
	}

	if len(postIds) > limit {
		postIds = postIds[:limit]
	}

	return getLobstersPostsFromIds(postIds)
}
