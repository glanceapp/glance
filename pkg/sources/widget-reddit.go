package sources

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

type redditSource struct {
	sourceBase          `yaml:",inline"`
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
		ID     string `yaml:"ID"`
		Secret string `yaml:"secret"`

		enabled        bool
		accessToken    string
		tokenExpiresAt time.Time
	} `yaml:"app-auth"`
}

func (s *redditSource) Feed() []Activity {
	activities := make([]Activity, len(s.Posts))
	for i, post := range s.Posts {
		activities[i] = post
	}
	return activities
}

func (s *redditSource) initialize() error {
	if s.Subreddit == "" {
		return errors.New("subreddit is required")
	}

	if s.Limit <= 0 {
		s.Limit = 15
	}

	if s.CollapseAfter == 0 || s.CollapseAfter < -1 {
		s.CollapseAfter = 5
	}

	sort := s.SortBy
	if sort != "hot" && sort != "new" && sort != "top" && sort != "rising" {
		s.SortBy = "hot"
	}

	p := s.TopPeriod
	if p != "hour" && p != "day" && p != "week" && p != "month" && p != "year" && p != "all" {
		s.TopPeriod = "day"
	}

	if s.RequestURLTemplate != "" {
		if !strings.Contains(s.RequestURLTemplate, "{REQUEST-URL}") {
			return errors.New("no `{REQUEST-URL}` placeholder specified")
		}
	}

	a := &s.AppAuth
	if a.Name != "" || a.ID != "" || a.Secret != "" {
		if a.Name == "" || a.ID == "" || a.Secret == "" {
			return errors.New("application name, client ID and client secret are required")
		}
		a.enabled = true
	}

	s.
		withTitle("r/" + s.Subreddit).
		withTitleURL("https://www.reddit.com/r/" + s.Subreddit + "/").
		withCacheDuration(30 * time.Minute)

	return nil
}

func (s *redditSource) update(ctx context.Context) {
	posts, err := s.fetchSubredditPosts()
	if !s.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(posts) > s.Limit {
		posts = posts[:s.Limit]
	}

	if s.ExtraSortBy == "engagement" {
		posts.calculateEngagement()
		posts.sortByEngagement()
	}

	s.Posts = posts
}

type subredditResponseJson struct {
	Data struct {
		Children []struct {
			Data struct {
				Id            string  `json:"ID"`
				Title         string  `json:"title"`
				SelfText      string  `json:"selftext"`
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
					Id        string `json:"ID"`
					Subreddit string `json:"subreddit"`
					Permalink string `json:"permalink"`
				} `json:"crosspost_parent_list"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

func (s *redditSource) parseCustomCommentsURL(subreddit, postId, postPath string) string {
	template := strings.ReplaceAll(s.CommentsURLTemplate, "{SUBREDDIT}", subreddit)
	template = strings.ReplaceAll(template, "{POST-ID}", postId)
	template = strings.ReplaceAll(template, "{POST-PATH}", strings.TrimLeft(postPath, "/"))

	return template
}

func (s *redditSource) fetchSubredditPosts() (forumPostList, error) {
	var client requestDoer = defaultHTTPClient
	var baseURL string
	var requestURL string
	var headers http.Header
	query := url.Values{}
	app := &s.AppAuth

	if !app.enabled {
		baseURL = "https://www.reddit.com"
		headers = http.Header{
			"User-Agent": []string{getBrowserUserAgentHeader()},
		}
	} else {
		baseURL = "https://oauth.reddit.com"

		if app.accessToken == "" || time.Now().Add(time.Minute).After(app.tokenExpiresAt) {
			if err := s.fetchNewAppAccessToken(); err != nil {
				return nil, fmt.Errorf("fetching new app access token: %v", err)
			}
		}

		headers = http.Header{
			"Authorization": []string{"Bearer " + app.accessToken},
			"User-Agent":    []string{app.Name + "/1.0"},
		}
	}

	if s.Limit > 25 {
		query.Set("limit", strconv.Itoa(s.Limit))
	}

	if s.Search != "" {
		query.Set("q", s.Search+" subreddit:"+s.Subreddit)
		query.Set("sort", s.SortBy)
		requestURL = fmt.Sprintf("%s/search.json?%s", baseURL, query.Encode())
	} else {
		if s.SortBy == "top" {
			query.Set("t", s.TopPeriod)
		}
		requestURL = fmt.Sprintf("%s/r/%s/%s.json?%s", baseURL, s.Subreddit, s.SortBy, query.Encode())
	}

	if s.RequestURLTemplate != "" {
		requestURL = strings.ReplaceAll(s.RequestURLTemplate, "{REQUEST-URL}", requestURL)
	} else if s.Proxy.client != nil {
		client = s.Proxy.client
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

		if s.CommentsURLTemplate == "" {
			commentsUrl = "https://www.reddit.com" + post.Permalink
		} else {
			commentsUrl = s.parseCustomCommentsURL(s.Subreddit, post.Id, post.Permalink)
		}

		forumPost := forumPost{
			ID:              post.Id,
			title:           html.UnescapeString(post.Title),
			Description:     post.SelfText,
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

		if s.ShowFlairs && post.Flair != "" {
			forumPost.Tags = append(forumPost.Tags, post.Flair)
		}

		if len(post.ParentList) > 0 {
			forumPost.IsCrosspost = true
			forumPost.TargetUrlDomain = "r/" + post.ParentList[0].Subreddit

			if s.CommentsURLTemplate == "" {
				forumPost.TargetUrl = "https://www.reddit.com" + post.ParentList[0].Permalink
			} else {
				forumPost.TargetUrl = s.parseCustomCommentsURL(
					post.ParentList[0].Subreddit,
					post.ParentList[0].Id,
					post.ParentList[0].Permalink,
				)
			}
		}

		if forumPost.TargetUrl != "" {
			article, err := readability.FromURL(forumPost.TargetUrl, 5*time.Second)
			if err == nil {
				forumPost.Description += fmt.Sprintf("\n\nReferenced article: \n%s", article.TextContent)
			} else {
				slog.Error("Failed to fetch reddit article", "error", err, "url", forumPost.TargetUrl)
			}
		}

		posts = append(posts, forumPost)
	}

	return posts, nil
}

func (s *redditSource) fetchNewAppAccessToken() error {
	body := strings.NewReader("grant_type=client_credentials")
	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", body)
	if err != nil {
		return fmt.Errorf("creating request for app access token: %v", err)
	}

	app := &s.AppAuth
	req.SetBasicAuth(app.ID, app.Secret)
	req.Header.Add("User-Agent", app.Name+"/1.0")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	type tokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	client := defaultHTTPClient
	if s.Proxy.client != nil {
		client = s.Proxy.client
	}
	response, err := decodeJsonFromRequest[tokenResponse](client, req)
	if err != nil {
		return err
	}

	app.accessToken = response.AccessToken
	app.tokenExpiresAt = time.Now().Add(time.Duration(response.ExpiresIn) * time.Second)

	return nil
}
