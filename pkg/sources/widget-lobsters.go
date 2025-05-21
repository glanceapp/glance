package sources

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

type lobstersSource struct {
	sourceBase     `yaml:",inline"`
	Posts          forumPostList `yaml:"-"`
	InstanceURL    string        `yaml:"instance-url"`
	CustomURL      string        `yaml:"custom-url"`
	Limit          int           `yaml:"limit"`
	CollapseAfter  int           `yaml:"collapse-after"`
	SortBy         string        `yaml:"sort-by"`
	Tags           []string      `yaml:"tags"`
	ShowThumbnails bool          `yaml:"-"`
}

func (s *lobstersSource) Feed() []Activity {
	activities := make([]Activity, len(s.Posts))
	for i, post := range s.Posts {
		activities[i] = post
	}
	return activities
}

func (s *lobstersSource) initialize() error {
	s.withTitle("Lobsters").withCacheDuration(time.Hour)

	if s.InstanceURL == "" {
		s.withTitleURL("https://lobste.rs")
	} else {
		s.withTitleURL(s.InstanceURL)
	}

	if s.SortBy == "" || (s.SortBy != "hot" && s.SortBy != "new") {
		s.SortBy = "hot"
	}

	if s.Limit <= 0 {
		s.Limit = 15
	}

	if s.CollapseAfter == 0 || s.CollapseAfter < -1 {
		s.CollapseAfter = 5
	}

	return nil
}

func (s *lobstersSource) update(ctx context.Context) {
	posts, err := fetchLobstersPosts(s.CustomURL, s.InstanceURL, s.SortBy, s.Tags)

	if !s.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if s.Limit < len(posts) {
		posts = posts[:s.Limit]
	}

	s.Posts = posts
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
			title:           post.Title,
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
