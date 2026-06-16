package glance

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
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

	Filters filterableFields[forumPost] `yaml:"filters"`

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

	posts = widget.Filters.Apply(posts)

	if widget.ExtraSortBy == "engagement" {
		posts.calculateEngagement()
		posts.sortByEngagement()
	}

	if len(posts) > widget.Limit {
		posts = posts[:widget.Limit]
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
	var client requestDoer = redditHTTPClient
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

	loid, err := getRedditLoidCookie()
	if err != nil {
		fmt.Printf("Error fetching reddit loid cookie: %v\n", err)
		return nil, errors.New("could not solve reddit challenge")
	}
	request.AddCookie(&http.Cookie{Name: "loid", Value: loid})

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

// On Windows the default HTTP client works fine, but on Linux it seems to
// get detected and blocked by reddit (or cloudflare) presumably because of TLS fingerprinting,
// so we use uTLS to mimic a real browser's TLS fingerprint, which seems to work around the issue
var redditHTTPClient = &http.Client{Transport: &http2.Transport{
	DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		tcpConn, err := (&net.Dialer{}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}

		uconn := utls.UClient(tcpConn, &utls.Config{
			ServerName: host,
		}, utls.HelloFirefox_Auto)

		if err := uconn.HandshakeContext(ctx); err != nil {
			tcpConn.Close()
			return nil, err
		}

		return uconn, nil
	},
}}

var (
	redditChallengePattern = regexp.MustCompile(`await\(async \w+\s*=>\s*\w+\s*\+\s*\w+\)\("([^"]+)"\)`)
	redditTokenPattern     = regexp.MustCompile(`name="token"\s+value="([^"]+)"`)
)

// Allows all widget instances to share a single loid cookie, since we don't want to draw
// too much attention by making a lot of requests to the flow that allows us to obtain
// the cookie required to access the .json endpoints
var getRedditLoidCookie = func() func() (string, error) {
	var lastUpdate time.Time
	var cachedLoid string

	return NewSingleflight(func() (string, error) {
		// Caching for 6 hours is a bit arbitrary, presumably the cookie is valid for 24 hours,
		// but we want to keep the cache time short in the event that a cookie becomes invalid
		// for whatever reason, since we don't have a way to force refresh it
		if time.Since(lastUpdate) < 6*time.Hour && cachedLoid != "" {
			return cachedLoid, nil
		}

		loid, err := fetchRedditLoidCookie()
		if err != nil {
			if cachedLoid != "" {
				fmt.Printf("Error fetching new reddit loid cookie, using cached value: %v\n", err)
				return cachedLoid, nil
			}
			return "", err
		}

		lastUpdate = time.Now()
		cachedLoid = loid
		return loid, nil
	})
}()

func fetchRedditLoidCookie() (string, error) {
	request, err := http.NewRequest("GET", "https://www.reddit.com/", nil)
	if err != nil {
		return "", err
	}

	setBrowserUserAgentHeader(request)

	response, err := redditHTTPClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d when requesting challenge page", response.StatusCode)
	}

	challengeBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	challengeMatches := redditChallengePattern.FindSubmatch(challengeBody)
	tokenMatches := redditTokenPattern.FindSubmatch(challengeBody)

	if challengeMatches == nil {
		return "", fmt.Errorf("no JS challenge found")
	}

	if tokenMatches == nil {
		return "", fmt.Errorf("no token found in challenge page")
	}

	challengeStr := string(challengeMatches[1])
	token := string(tokenMatches[1])
	solution := challengeStr + challengeStr // the JS does: e + e

	params := url.Values{
		"solution":     {solution},
		"js_challenge": {"1"},
		"token":        {token},
	}
	request, err = http.NewRequest("GET", "https://www.reddit.com/?"+params.Encode(), nil)
	if err != nil {
		return "", err
	}

	setBrowserUserAgentHeader(request)

	response, err = redditHTTPClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d when submitting challenge solution", response.StatusCode)
	}

	for _, cookie := range response.Cookies() {
		if cookie.Name == "loid" {
			return cookie.Value, nil
		}
	}

	return "", fmt.Errorf("no loid cookie found")
}
