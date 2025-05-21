package widgets

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

type lobstersWidget struct {
	widgetBase     `yaml:",inline"`
	Posts          forumPostList `yaml:"-"`
	InstanceURL    string        `yaml:"instance-url"`
	CustomURL      string        `yaml:"custom-url"`
	Limit          int           `yaml:"limit"`
	CollapseAfter  int           `yaml:"collapse-after"`
	SortBy         string        `yaml:"sort-by"`
	Tags           []string      `yaml:"tags"`
	ShowThumbnails bool          `yaml:"-"`
}

func (widget *lobstersWidget) initialize() error {
	widget.withTitle("Lobsters").withCacheDuration(time.Hour)

	if widget.InstanceURL == "" {
		widget.withTitleURL("https://lobste.rs")
	} else {
		widget.withTitleURL(widget.InstanceURL)
	}

	if widget.SortBy == "" || (widget.SortBy != "hot" && widget.SortBy != "new") {
		widget.SortBy = "hot"
	}

	if widget.Limit <= 0 {
		widget.Limit = 15
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	return nil
}

func (widget *lobstersWidget) update(ctx context.Context) {
	posts, err := fetchLobstersPosts(widget.CustomURL, widget.InstanceURL, widget.SortBy, widget.Tags)

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

func (widget *lobstersWidget) rankByRelevancy(query string) {
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
			ImageURL:    "",
			PublishedAt: e.TimePosted,
		})
	}

	matches, err := llm.filterFeed(context.Background(), feed, query)
	if err != nil {
		slog.Error("Failed to filter lobsters posts", "error", err)
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

func (widget *lobstersWidget) Render() template.HTML {
	return widget.renderTemplate(widget, forumPostsTemplate)
}

type lobstersPostResponseJson struct {
	ID           string   `json:"short_id"`
	CreatedAt    string   `json:"created_at"`
	Title        string   `json:"title"`
	URL          string   `json:"url"`
	Score        int      `json:"score"`
	CommentCount int      `json:"comment_count"`
	CommentsURL  string   `json:"comments_url"`
	Tags         []string `json:"tags"`
}

type lobstersFeedResponseJson []lobstersPostResponseJson

func fetchLobstersPostsFromFeed(feedUrl string) (forumPostList, error) {
	request, err := http.NewRequest("GET", feedUrl, nil)
	if err != nil {
		return nil, err
	}

	feed, err := decodeJsonFromRequest[lobstersFeedResponseJson](defaultHTTPClient, request)
	if err != nil {
		return nil, err
	}

	posts := make(forumPostList, 0, len(feed))

	for _, post := range feed {
		createdAt, _ := time.Parse(time.RFC3339, post.CreatedAt)

		forumPost := forumPost{
			ID:              post.ID,
			Title:           post.Title,
			Description:     post.Title,
			DiscussionUrl:   post.CommentsURL,
			TargetUrl:       post.URL,
			TargetUrlDomain: extractDomainFromUrl(post.URL),
			CommentCount:    post.CommentCount,
			Score:           post.Score,
			TimePosted:      createdAt,
			Tags:            post.Tags,
		}

		article, err := readability.FromURL(post.URL, 5*time.Second)
		if err == nil {
			forumPost.Description = article.TextContent
		} else {
			slog.Error("Failed to fetch lobster article", "error", err, "url", forumPost.TargetUrl)
		}

		posts = append(posts, forumPost)
	}

	if len(posts) == 0 {
		return nil, errNoContent
	}

	return posts, nil
}

func fetchLobstersPosts(customURL string, instanceURL string, sortBy string, tags []string) (forumPostList, error) {
	var feedUrl string

	if customURL != "" {
		feedUrl = customURL
	} else {
		if instanceURL != "" {
			instanceURL = strings.TrimRight(instanceURL, "/") + "/"
		} else {
			instanceURL = "https://lobste.rs/"
		}

		if sortBy == "hot" {
			sortBy = "hottest"
		} else if sortBy == "new" {
			sortBy = "newest"
		}

		if len(tags) == 0 {
			feedUrl = instanceURL + sortBy + ".json"
		} else {
			tags := strings.Join(tags, ",")
			feedUrl = instanceURL + "t/" + tags + ".json"
		}
	}

	posts, err := fetchLobstersPostsFromFeed(feedUrl)
	if err != nil {
		return nil, err
	}

	return posts, nil
}
