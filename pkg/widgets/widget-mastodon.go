package widgets

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"
)

type mastodonWidget struct {
	widgetBase     `yaml:",inline"`
	Posts          forumPostList `yaml:"-"`
	InstanceURL    string        `yaml:"instance-url"`
	Accounts       []string      `yaml:"accounts"`
	Hashtags       []string      `yaml:"hashtags"`
	Limit          int           `yaml:"limit"`
	CollapseAfter  int           `yaml:"collapse-after"`
	ShowThumbnails bool          `yaml:"-"`
}

func (widget *mastodonWidget) initialize() error {
	if widget.InstanceURL == "" {
		return fmt.Errorf("instance-url is required")
	}

	widget.
		withTitle("Mastodon").
		withTitleURL(widget.InstanceURL).
		withCacheDuration(30 * time.Minute)

	if widget.Limit <= 0 {
		widget.Limit = 15
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	return nil
}

func (widget *mastodonWidget) update(ctx context.Context) {
	posts, err := fetchMastodonPosts(widget.InstanceURL, widget.Accounts, widget.Hashtags)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if widget.Limit < len(posts) {
		posts = posts[:widget.Limit]
	}

	widget.Posts = posts

	if widget.filterQuery != "" {
		widget.rankByRelevancy(widget.filterQuery)
	}
}

func (widget *mastodonWidget) Render() template.HTML {
	return widget.renderTemplate(widget, forumPostsTemplate)
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

func (widget *mastodonWidget) rankByRelevancy(query string) {
	llm, err := NewLLM()
	if err != nil {
		slog.Error("Failed to initialize LLM", "error", err)
		return
	}

	feed := make([]feedEntry, 0, len(widget.Posts))
	for _, e := range widget.Posts {
		feed = append(feed, feedEntry{
			ID:          e.ID,
			Title:       e.Title,
			Description: e.Description,
			URL:         e.TargetUrl,
			ImageURL:    e.ThumbnailUrl,
			PublishedAt: e.TimePosted,
		})
	}

	matches, err := llm.filterFeed(context.Background(), feed, query)
	if err != nil {
		slog.Error("Failed to filter Mastodon posts", "error", err)
		return
	}

	matchesMap := make(map[string]feedMatch)
	for _, match := range matches {
		matchesMap[match.ID] = match
	}

	filtered := make(forumPostList, 0, len(matches))
	for _, e := range widget.Posts {
		if match, ok := matchesMap[e.ID]; ok {
			e.MatchSummary = match.Highlight
			e.MatchScore = match.Score
			filtered = append(filtered, e)
		}
	}

	widget.Posts = filtered
}
