package sources

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"
)

type mastodonSource struct {
	sourceBase     `yaml:",inline"`
	Posts          forumPostList `yaml:"-"`
	InstanceURL    string        `yaml:"instance-url"`
	Accounts       []string      `yaml:"accounts"`
	Hashtags       []string      `yaml:"hashtags"`
	Limit          int           `yaml:"limit"`
	CollapseAfter  int           `yaml:"collapse-after"`
	ShowThumbnails bool          `yaml:"-"`
}

func (s *mastodonSource) initialize() error {
	if s.InstanceURL == "" {
		return fmt.Errorf("instance-url is required")
	}

	s.
		withTitle("Mastodon").
		withTitleURL(s.InstanceURL).
		withCacheDuration(30 * time.Minute)

	if s.Limit <= 0 {
		s.Limit = 15
	}

	if s.CollapseAfter == 0 || s.CollapseAfter < -1 {
		s.CollapseAfter = 5
	}

	return nil
}

func (s *mastodonSource) update(ctx context.Context) {
	posts, err := fetchMastodonPosts(s.InstanceURL, s.Accounts, s.Hashtags)

	if !s.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if s.Limit < len(posts) {
		posts = posts[:s.Limit]
	}

	s.Posts = posts
}

type mastodonPostResponseJson struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
	Reblogs   int       `json:"reblogs_count"`
	Favorites int       `json:"favourites_count"`
	Replies   int       `json:"replies_count"`
	Account   struct {
		Username string `json:"username"`
		URL      string `json:"url"`
	} `json:"account"`
	MediaAttachments []struct {
		URL string `json:"url"`
	} `json:"media_attachments"`
	Tags []struct {
		Name string `json:"name"`
	} `json:"tags"`
}

func fetchMastodonPosts(instanceURL string, accounts []string, hashtags []string) (forumPostList, error) {
	instanceURL = strings.TrimRight(instanceURL, "/")
	var posts forumPostList

	// Fetch posts from specified accounts
	for _, account := range accounts {
		url := fmt.Sprintf("%s/api/v1/accounts/%s/statuses", instanceURL, account)
		request, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		accountPosts, err := decodeJsonFromRequest[[]mastodonPostResponseJson](defaultHTTPClient, request)
		if err != nil {
			slog.Error("Failed to fetch Mastodon account posts", "error", err, "account", account)
			continue
		}

		for _, post := range accountPosts {
			forumPost := convertMastodonPostToForumPost(post)
			posts = append(posts, forumPost)
		}
	}

	// Fetch posts from specified hashtags
	for _, hashtag := range hashtags {
		url := fmt.Sprintf("%s/api/v1/timelines/tag/%s", instanceURL, hashtag)
		request, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		hashtagPosts, err := decodeJsonFromRequest[[]mastodonPostResponseJson](defaultHTTPClient, request)
		if err != nil {
			slog.Error("Failed to fetch Mastodon hashtag posts", "error", err, "hashtag", hashtag)
			continue
		}

		for _, post := range hashtagPosts {
			forumPost := convertMastodonPostToForumPost(post)
			posts = append(posts, forumPost)
		}
	}

	if len(posts) == 0 {
		return nil, errNoContent
	}

	return posts, nil
}

func convertMastodonPostToForumPost(post mastodonPostResponseJson) forumPost {
	tags := make([]string, len(post.Tags))
	for i, tag := range post.Tags {
		tags[i] = "#" + tag.Name
	}

	plainText := extractTextFromHTML(post.Content)
	title := oneLineTitle(plainText, 50)

	forumPost := forumPost{
		ID:            post.ID,
		Title:         title,
		Description:   plainText,
		DiscussionUrl: post.URL,
		CommentCount:  post.Replies,
		Score:         post.Reblogs + post.Favorites,
		TimePosted:    post.CreatedAt,
		// TODO(pulse): Hide tags for now, as they introduce too much noise
		// Tags:          tags,
	}

	if len(post.MediaAttachments) > 0 {
		forumPost.ThumbnailUrl = post.MediaAttachments[0].URL
	}

	return forumPost
}

func extractTextFromHTML(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return htmlStr
	}
	var b strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return strings.TrimSpace(b.String())
}

func oneLineTitle(text string, maxLen int) string {
	// Replace newlines and tabs with spaces, collapse multiple spaces
	re := regexp.MustCompile(`\s+`)
	t := re.ReplaceAllString(text, " ")
	t = strings.TrimSpace(t)
	if utf8.RuneCountInString(t) > maxLen {
		runes := []rune(t)
		return string(runes[:maxLen-1]) + "…"
	}
	return t
}
