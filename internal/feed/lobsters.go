package feed

import (
	"net/http"
	"strings"
	"time"
)

type lobstersPostResponseJson struct {
	CreatedAt    string   `json:"created_at"`
	Title        string   `json:"title"`
	URL          string   `json:"url"`
	Score        int      `json:"score"`
	CommentCount int      `json:"comment_count"`
	CommentsURL  string   `json:"comments_url"`
	Tags         []string `json:"tags"`
}

type lobstersFeedResponseJson []lobstersPostResponseJson

func getLobstersPostsFromFeed(feedUrl string) (ForumPosts, error) {
	request, err := http.NewRequest("GET", feedUrl, nil)

	if err != nil {
		return nil, err
	}

	feed, err := decodeJsonFromRequest[lobstersFeedResponseJson](defaultClient, request)

	if err != nil {
		return nil, err
	}

	posts := make(ForumPosts, 0, len(feed))

	for i := range feed {
		createdAt, _ := time.Parse(time.RFC3339, feed[i].CreatedAt)

		posts = append(posts, ForumPost{
			Title:           feed[i].Title,
			DiscussionUrl:   feed[i].CommentsURL,
			TargetUrl:       feed[i].URL,
			TargetUrlDomain: extractDomainFromUrl(feed[i].URL),
			CommentCount:    feed[i].CommentCount,
			Score:           feed[i].Score,
			TimePosted:      createdAt,
			Tags:            feed[i].Tags,
		})
	}

	if len(posts) == 0 {
		return nil, ErrNoContent
	}

	return posts, nil
}

func FetchLobstersPosts(sortBy string, tags []string) (ForumPosts, error) {
	var feedUrl string

	if sortBy == "hot" {
		sortBy = "hottest"
	} else if sortBy == "new" {
		sortBy = "newest"
	}

	if len(tags) == 0 {
		feedUrl = "https://lobste.rs/" + sortBy + ".json"
	} else {
		tags := strings.Join(tags, ",")
		feedUrl = "https://lobste.rs/t/" + tags + ".json"
	}

	posts, err := getLobstersPostsFromFeed(feedUrl)

	if err != nil {
		return nil, err
	}

	return posts, nil
}
