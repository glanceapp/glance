package glance

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	redditWidgetHorizontalCardsTemplate = mustParseTemplate("reddit-horizontal-cards.html", "widget-base.html")
	redditWidgetVerticalCardsTemplate   = mustParseTemplate("reddit-vertical-cards.html", "widget-base.html")
)

var ErrAccessTokenMissingParams = errors.New("application name, client ID and client secret are required to get a Reddit access token")

type redditWidget struct {
	widgetBase                 `yaml:",inline"`
	Posts                      forumPostList     `yaml:"-"`
	Subreddit                  string            `yaml:"subreddit"`
	Proxy                      proxyOptionsField `yaml:"proxy"`
	Style                      string            `yaml:"style"`
	ShowThumbnails             bool              `yaml:"show-thumbnails"`
	ShowFlairs                 bool              `yaml:"show-flairs"`
	SortBy                     string            `yaml:"sort-by"`
	TopPeriod                  string            `yaml:"top-period"`
	Search                     string            `yaml:"search"`
	ExtraSortBy                string            `yaml:"extra-sort-by"`
	CommentsUrlTemplate        string            `yaml:"comments-url-template"`
	Limit                      int               `yaml:"limit"`
	CollapseAfter              int               `yaml:"collapse-after"`
	RequestUrlTemplate         string            `yaml:"request-url-template"`
	RedditAppName              string            `yaml:"reddit-app-name"`
	RedditClientID             string            `yaml:"reddit-client-id"`
	RedditClientSecret         string            `yaml:"reddit-client-secret"`
	redditAccessToken          string
	redditAccessTokenExpiresAt time.Time
}

type redditTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
}

func (widget *redditWidget) initialize() error {
	if widget.Subreddit == "" {
		return errors.New("subreddit is required")
	}

	if widget.Limit <= 0 {
		widget.Limit = 15
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	if !isValidRedditSortType(widget.SortBy) {
		widget.SortBy = "hot"
	}

	if !isValidRedditTopPeriod(widget.TopPeriod) {
		widget.TopPeriod = "day"
	}

	if widget.RequestUrlTemplate != "" {
		if !strings.Contains(widget.RequestUrlTemplate, "{REQUEST-URL}") {
			return errors.New("no `{REQUEST-URL}` placeholder specified")
		}
	}

	widget.
		withTitle("r/" + widget.Subreddit).
		withTitleURL("https://www.reddit.com/r/" + widget.Subreddit + "/").
		withCacheDuration(30 * time.Minute)

	return nil
}

func isValidRedditSortType(sortBy string) bool {
	return sortBy == "hot" ||
		sortBy == "new" ||
		sortBy == "top" ||
		sortBy == "rising"
}

func isValidRedditTopPeriod(period string) bool {
	return period == "hour" ||
		period == "day" ||
		period == "week" ||
		period == "month" ||
		period == "year" ||
		period == "all"
}

func (widget *redditWidget) update(ctx context.Context) {
	posts, err := widget.fetchSubredditPosts()

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(posts) > widget.Limit {
		posts = posts[:widget.Limit]
	}

	if widget.ExtraSortBy == "engagement" {
		posts.calculateEngagement()
		posts.sortByEngagement()
	}

	widget.Posts = posts
}

func (widget *redditWidget) Render() template.HTML {
	if widget.Style == "horizontal-cards" {
		return widget.renderTemplate(widget, redditWidgetHorizontalCardsTemplate)
	}

	if widget.Style == "vertical-cards" {
		return widget.renderTemplate(widget, redditWidgetVerticalCardsTemplate)
	}

	return widget.renderTemplate(widget, forumPostsTemplate)

}

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
				Flair         string  `json:"link_flair_text"`
				ParentList    []struct {
					Id        string `json:"id"`
					Subreddit string `json:"subreddit"`
					Permalink string `json:"permalink"`
				} `json:"crosspost_parent_list"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

func templateRedditCommentsURL(template, subreddit, postId, postPath string) string {
	template = strings.ReplaceAll(template, "{SUBREDDIT}", subreddit)
	template = strings.ReplaceAll(template, "{POST-ID}", postId)
	template = strings.ReplaceAll(template, "{POST-PATH}", strings.TrimLeft(postPath, "/"))

	return template
}

func (widget *redditWidget) fetchSubredditPosts() (forumPostList, error) {
	var baseURL string

	accessToken, err := widget.getRedditAccessToken()
	if err != nil {
		return nil, fmt.Errorf("getting Reddit access token: %w", err)
	}

	if accessToken != "" {
		baseURL = "https://oauth.reddit.com"
	} else {
		baseURL = "https://www.reddit.com"
	}

	query := url.Values{}
	var requestURL string

	if widget.Search != "" {
		query.Set("q", widget.Search+" subreddit:"+widget.Subreddit)
		query.Set("sort", widget.SortBy)

		requestURL = fmt.Sprintf("%s/search.json?%s", baseURL, query.Encode())
	} else {
		if widget.SortBy == "top" {
			query.Set("t", widget.TopPeriod)
		}

		requestURL = fmt.Sprintf("%s/r/%s/%s.json?%s", baseURL, widget.Subreddit, widget.SortBy, query.Encode())
	}

	var client requestDoer = defaultHTTPClient

	if widget.RequestUrlTemplate != "" {
		requestURL = strings.ReplaceAll(widget.RequestUrlTemplate, "{REQUEST-URL}", requestURL)
	} else if widget.Proxy.client != nil {
		client = widget.Proxy.client
	}

	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}

	// Required to increase rate limit, otherwise Reddit randomly returns 429 even after just 2 requests
	if widget.RedditAppName != "" {
		request.Header.Set("User-Agent", fmt.Sprintf("%s/1.0", widget.RedditAppName))
	} else {
		setBrowserUserAgentHeader(request)
	}

	if widget.redditAccessToken != "" {
		request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	}

	responseJson, err := decodeJsonFromRequest[subredditResponseJson](client, request)
	if err != nil {
		return nil, err
	}

	if len(responseJson.Data.Children) == 0 {
		return nil, fmt.Errorf("no posts found")
	}

	posts := make(forumPostList, 0, len(responseJson.Data.Children))

	for i := range responseJson.Data.Children {
		post := &responseJson.Data.Children[i].Data

		if post.Stickied || post.Pinned {
			continue
		}

		var commentsUrl string

		if widget.CommentsUrlTemplate == "" {
			commentsUrl = "https://www.reddit.com" + post.Permalink
		} else {
			commentsUrl = templateRedditCommentsURL(widget.CommentsUrlTemplate, widget.Subreddit, post.Id, post.Permalink)
		}

		forumPost := forumPost{
			Title:           html.UnescapeString(post.Title),
			DiscussionUrl:   commentsUrl,
			TargetUrlDomain: post.Domain,
			CommentCount:    post.CommentsCount,
			Score:           post.Upvotes,
			TimePosted:      time.Unix(int64(post.Time), 0),
		}

		if post.Thumbnail != "" && post.Thumbnail != "self" && post.Thumbnail != "default" && post.Thumbnail != "nsfw" {
			forumPost.ThumbnailUrl = html.UnescapeString(post.Thumbnail)
		}

		if !post.IsSelf {
			forumPost.TargetUrl = post.Url
		}

		if widget.ShowFlairs && post.Flair != "" {
			forumPost.Tags = append(forumPost.Tags, post.Flair)
		}

		if len(post.ParentList) > 0 {
			forumPost.IsCrosspost = true
			forumPost.TargetUrlDomain = "r/" + post.ParentList[0].Subreddit

			if widget.CommentsUrlTemplate == "" {
				forumPost.TargetUrl = "https://www.reddit.com" + post.ParentList[0].Permalink
			} else {
				forumPost.TargetUrl = templateRedditCommentsURL(
					widget.CommentsUrlTemplate,
					post.ParentList[0].Subreddit,
					post.ParentList[0].Id,
					post.ParentList[0].Permalink,
				)
			}
		}

		posts = append(posts, forumPost)
	}

	return posts, nil
}

func (widget *redditWidget) queryRedditAPIForAccessToken() (err error) {
	if widget.RedditAppName == "" || widget.RedditClientID == "" || widget.RedditClientSecret == "" {
		return ErrAccessTokenMissingParams
	}

	auth := base64.StdEncoding.EncodeToString([]byte(widget.RedditClientID + ":" + widget.RedditClientSecret))

	data := url.Values{"grant_type": {"client_credentials"}}

	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("requesting an access token to the Reddit API: %w", err)
	}

	req.Header.Add("Authorization", "Basic "+auth)
	req.Header.Add("User-Agent", widget.RedditAppName+"/1.0")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("querying Reddit API: %w", err)
	}

	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp redditTokenResponse
	err = json.Unmarshal(body, &tokenResp)
	if err != nil {
		return fmt.Errorf("unmarshalling Reddit API response: %w", err)
	}

	widget.redditAccessToken = tokenResp.AccessToken
	widget.redditAccessTokenExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return
}

// getRedditAccessToken checks if an unexpired Reddit access token is present, if not, it fetches one and returns it.
func (widget *redditWidget) getRedditAccessToken() (string, error) {
	// If parameters to query the Reddit API for an access token are missing, return nothing.
	if widget.RedditAppName == "" || widget.RedditClientID == "" || widget.RedditClientSecret == "" {
		return "", nil
	}

	// Check if the token is still valid in a minute (gives a margin to avoid authentication failure)
	if widget.redditAccessToken != "" && time.Now().Add(time.Minute).Before(widget.redditAccessTokenExpiresAt) {
		return widget.redditAccessToken, nil
	}

	err := widget.queryRedditAPIForAccessToken()
	return widget.redditAccessToken, err
}
