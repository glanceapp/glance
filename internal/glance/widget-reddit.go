package glance

import (
	"context"
	"errors"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	redditWidgetHorizontalCardsTemplate = mustParseTemplate("reddit-horizontal-cards.html", "widget-base.html")
	redditWidgetVerticalCardsTemplate   = mustParseTemplate("reddit-vertical-cards.html", "widget-base.html")
)

type redditWidget struct {
	widgetBase          `yaml:",inline"`
	Posts               forumPostList     `yaml:"-"`
	Subreddit           string            `yaml:"subreddit"`
	Proxy               proxyOptionsField `yaml:"proxy"`
	Style               string            `yaml:"style"`
	ShowThumbnails      bool              `yaml:"show-thumbnails"`
	ShowFlairs          bool              `yaml:"show-flairs"`
	SortBy              string            `yaml:"sort-by"`
	TopPeriod           string            `yaml:"top-period"`
	Search              string            `yaml:"search"`
	ExtraSortBy         string            `yaml:"extra-sort-by"`
	CommentsURLTemplate string            `yaml:"comments-url-template"`
	Limit               int               `yaml:"limit"`
	CollapseAfter       int               `yaml:"collapse-after"`
	RequestURLTemplate  string            `yaml:"request-url-template"`

	AppAuth struct {
		Name   string `yaml:"name"`
		ID     string `yaml:"id"`
		Secret string `yaml:"secret"`

		enabled        bool
		accessToken    string
		tokenExpiresAt time.Time
	} `yaml:"app-auth"`
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

	s := widget.SortBy
	if s != "hot" && s != "new" && s != "top" && s != "rising" {
		widget.SortBy = "hot"
	}

	p := widget.TopPeriod
	if p != "hour" && p != "day" && p != "week" && p != "month" && p != "year" && p != "all" {
		widget.TopPeriod = "day"
	}

	if widget.RequestURLTemplate != "" {
		if !strings.Contains(widget.RequestURLTemplate, "{REQUEST-URL}") {
			return errors.New("no `{REQUEST-URL}` placeholder specified")
		}
	}

	a := &widget.AppAuth
	if a.Name != "" || a.ID != "" || a.Secret != "" {
		if a.Name == "" || a.ID == "" || a.Secret == "" {
			return errors.New("application name, client ID and client secret are required")
		}
		a.enabled = true
	}

	widget.
		withTitle("r/" + widget.Subreddit).
		withTitleURL("https://www.reddit.com/r/" + widget.Subreddit + "/").
		withCacheDuration(30 * time.Minute)

	return nil
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

func (widget *redditWidget) parseCustomCommentsURL(subreddit, postId, postPath string) string {
	template := strings.ReplaceAll(widget.CommentsURLTemplate, "{SUBREDDIT}", subreddit)
	template = strings.ReplaceAll(template, "{POST-ID}", postId)
	template = strings.ReplaceAll(template, "{POST-PATH}", strings.TrimLeft(postPath, "/"))

	return template
}

func (widget *redditWidget) fetchSubredditPosts() (forumPostList, error) {
	var client requestDoer = defaultHTTPClient
	var baseURL string
	var requestURL string
	var headers http.Header
	query := url.Values{}
	app := &widget.AppAuth

	if !app.enabled {
		baseURL = "https://www.reddit.com"
		headers = http.Header{
			"User-Agent": []string{getBrowserUserAgentHeader()},
		}
	} else {
		baseURL = "https://oauth.reddit.com"

		if app.accessToken == "" || time.Now().Add(time.Minute).After(app.tokenExpiresAt) {
			if err := widget.fetchNewAppAccessToken(); err != nil {
				return nil, fmt.Errorf("fetching new app access token: %v", err)
			}
		}

		headers = http.Header{
			"Authorization": []string{"Bearer " + app.accessToken},
			"User-Agent":    []string{app.Name + "/1.0"},
		}
	}

	if widget.Limit > 25 {
		query.Set("limit", strconv.Itoa(widget.Limit))
	}

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

	if widget.RequestURLTemplate != "" {
		requestURL = strings.ReplaceAll(widget.RequestURLTemplate, "{REQUEST-URL}", requestURL)
	} else if widget.Proxy.client != nil {
		client = widget.Proxy.client
	}

	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header = headers

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

		if widget.CommentsURLTemplate == "" {
			commentsUrl = "https://www.reddit.com" + post.Permalink
		} else {
			commentsUrl = widget.parseCustomCommentsURL(widget.Subreddit, post.Id, post.Permalink)
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

			if widget.CommentsURLTemplate == "" {
				forumPost.TargetUrl = "https://www.reddit.com" + post.ParentList[0].Permalink
			} else {
				forumPost.TargetUrl = widget.parseCustomCommentsURL(
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

func (widget *redditWidget) fetchNewAppAccessToken() error {
	body := strings.NewReader("grant_type=client_credentials")
	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", body)
	if err != nil {
		return fmt.Errorf("creating request for app access token: %v", err)
	}

	app := &widget.AppAuth
	req.SetBasicAuth(app.ID, app.Secret)
	req.Header.Add("User-Agent", app.Name+"/1.0")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	type tokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	client := ternary(widget.Proxy.client != nil, widget.Proxy.client, defaultHTTPClient)
	response, err := decodeJsonFromRequest[tokenResponse](client, req)
	if err != nil {
		return err
	}

	app.accessToken = response.AccessToken
	app.tokenExpiresAt = time.Now().Add(time.Duration(response.ExpiresIn) * time.Second)

	return nil
}
