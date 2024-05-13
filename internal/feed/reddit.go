package feed

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type subredditResponseJson struct {
	Data struct {
		Children []struct {
			Data struct {
				Id            string  `json:"id"`
				Title         string  `json:"title"`
				Upvotes       int     `json:"ups"`
				Url           string  `json:"url"`
				Time          float64 `json:"created"`
				CommentsCount int     `json:"num_comments"`
				Domain        string  `json:"domain"`
				Permalink     string  `json:"permalink"`
				Stickied      bool    `json:"stickied"`
				Pinned        bool    `json:"pinned"`
				IsSelf        bool    `json:"is_self"`
				Thumbnail     string  `json:"thumbnail"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

func FetchSubredditPosts(subreddit, sort, topPeriod, search, commentsUrlTemplate, requestUrlTemplate string) (ForumPosts, error) {
	query := url.Values{}
	var requestUrl string

	if search != "" {
		query.Set("q", search+" subreddit:"+subreddit)
		query.Set("sort", sort)
	}

	if sort == "top" {
		query.Set("t", topPeriod)
	}

	if search != "" {
		requestUrl = fmt.Sprintf("https://www.reddit.com/search.json?%s", query.Encode())
	} else {
		requestUrl = fmt.Sprintf("https://www.reddit.com/r/%s/%s.json?%s", subreddit, sort, query.Encode())
	}

	if requestUrlTemplate != "" {
		requestUrl = strings.ReplaceAll(requestUrlTemplate, "{REQUEST-URL}", requestUrl)
	}

	request, err := http.NewRequest("GET", requestUrl, nil)

	if err != nil {
		return nil, err
	}

	// Required to increase rate limit, otherwise Reddit randomly returns 429 even after just 2 requests
	addBrowserUserAgentHeader(request)
	responseJson, err := decodeJsonFromRequest[subredditResponseJson](defaultClient, request)

	if err != nil {
		return nil, err
	}

	if len(responseJson.Data.Children) == 0 {
		return nil, fmt.Errorf("no posts found")
	}

	posts := make(ForumPosts, 0, len(responseJson.Data.Children))

	for i := range responseJson.Data.Children {
		post := &responseJson.Data.Children[i].Data

		if post.Stickied || post.Pinned {
			continue
		}

		var commentsUrl string

		if commentsUrlTemplate == "" {
			commentsUrl = "https://www.reddit.com" + post.Permalink
		} else {
			commentsUrl = strings.ReplaceAll(commentsUrlTemplate, "{SUBREDDIT}", subreddit)
			commentsUrl = strings.ReplaceAll(commentsUrl, "{POST-ID}", post.Id)
			commentsUrl = strings.ReplaceAll(commentsUrl, "{POST-PATH}", strings.TrimLeft(post.Permalink, "/"))
		}

		forumPost := ForumPost{
			Title:           html.UnescapeString(post.Title),
			DiscussionUrl:   commentsUrl,
			TargetUrlDomain: post.Domain,
			CommentCount:    post.CommentsCount,
			Score:           post.Upvotes,
			TimePosted:      time.Unix(int64(post.Time), 0),
		}

		if post.Thumbnail != "" && post.Thumbnail != "self" && post.Thumbnail != "default" {
			forumPost.ThumbnailUrl = post.Thumbnail
		}

		if !post.IsSelf {
			forumPost.TargetUrl = post.Url
		}

		posts = append(posts, forumPost)
	}

	return posts, nil
}
