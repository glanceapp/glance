package glance

import (
	"context"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	gofeedext "github.com/mmcdole/gofeed/extensions"
)

var (
	rssWidgetTemplate                 = mustParseTemplate("rss-list.html", "widget-base.html")
	rssWidgetDetailedListTemplate     = mustParseTemplate("rss-detailed-list.html", "widget-base.html")
	rssWidgetHorizontalCardsTemplate  = mustParseTemplate("rss-horizontal-cards.html", "widget-base.html")
	rssWidgetHorizontalCards2Template = mustParseTemplate("rss-horizontal-cards-2.html", "widget-base.html")
)

type cachedFeed struct {
	LastModified time.Time
	Etag         string
	Items        rssFeedItemList
}

type rssWidget struct {
	widgetBase       `yaml:",inline"`
	FeedRequests     []rssFeedRequest      `yaml:"feeds"`
	Style            string                `yaml:"style"`
	ThumbnailHeight  float64               `yaml:"thumbnail-height"`
	CardHeight       float64               `yaml:"card-height"`
	Items            rssFeedItemList       `yaml:"-"`
	Limit            int                   `yaml:"limit"`
	CollapseAfter    int                   `yaml:"collapse-after"`
	SingleLineTitles bool                  `yaml:"single-line-titles"`
	PreserveOrder    bool                  `yaml:"preserve-order"`
	NoItemsMessage   string                `yaml:"-"`
	CachedFeeds      map[string]cachedFeed `yaml:"-"`
}

func (widget *rssWidget) initialize() error {
	widget.withTitle("RSS Feed").withCacheDuration(1 * time.Hour)

	if widget.Limit <= 0 {
		widget.Limit = 25
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	if widget.ThumbnailHeight < 0 {
		widget.ThumbnailHeight = 0
	}

	if widget.CardHeight < 0 {
		widget.CardHeight = 0
	}

	if widget.Style == "detailed-list" {
		for i := range widget.FeedRequests {
			widget.FeedRequests[i].IsDetailed = true
		}
	}

	widget.NoItemsMessage = "No items were returned from the feeds."

	return nil
}

func (widget *rssWidget) update(ctx context.Context) {
	// Populate If-Modified-Since header and Etag
	for i, req := range widget.FeedRequests {
		if cachedFeed, ok := widget.CachedFeeds[req.URL]; ok {
			widget.FeedRequests[i].IfModifiedSince = cachedFeed.LastModified
			widget.FeedRequests[i].Etag = cachedFeed.Etag
		}
	}

	allItems, feeds, err := fetchItemsFromRSSFeeds(widget.FeedRequests, widget.CachedFeeds)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if !widget.PreserveOrder {
		allItems.sortByNewest()
	}

	if len(allItems) > widget.Limit {
		allItems = allItems[:widget.Limit]
	}

	widget.Items = allItems

	cachedFeeds := make(map[string]cachedFeed)
	for _, feed := range feeds {
		if !feed.LastModified.IsZero() || feed.Etag != "" {
			cachedFeeds[feed.URL] = cachedFeed{
				LastModified: feed.LastModified,
				Etag:         feed.Etag,
				Items:        feed.Items,
			}
		}
	}
	widget.CachedFeeds = cachedFeeds
}

func (widget *rssWidget) Render() template.HTML {
	if widget.Style == "horizontal-cards" {
		return widget.renderTemplate(widget, rssWidgetHorizontalCardsTemplate)
	}

	if widget.Style == "horizontal-cards-2" {
		return widget.renderTemplate(widget, rssWidgetHorizontalCards2Template)
	}

	if widget.Style == "detailed-list" {
		return widget.renderTemplate(widget, rssWidgetDetailedListTemplate)
	}

	return widget.renderTemplate(widget, rssWidgetTemplate)
}

type rssFeedItem struct {
	ChannelName string
	ChannelURL  string
	Title       string
	Link        string
	ImageURL    string
	Categories  []string
	Description string
	PublishedAt time.Time
}

// doesn't cover all cases but works the vast majority of the time
var htmlTagsWithAttributesPattern = regexp.MustCompile(`<\/?[a-zA-Z0-9-]+ *(?:[a-zA-Z-]+=(?:"|').*?(?:"|') ?)* *\/?>`)

func sanitizeFeedDescription(description string) string {
	if description == "" {
		return ""
	}

	description = strings.ReplaceAll(description, "\n", " ")
	description = htmlTagsWithAttributesPattern.ReplaceAllString(description, "")
	description = sequentialWhitespacePattern.ReplaceAllString(description, " ")
	description = strings.TrimSpace(description)
	description = html.UnescapeString(description)

	return description
}

func shortenFeedDescriptionLen(description string, maxLen int) string {
	description, _ = limitStringLength(description, 1000)
	description = sanitizeFeedDescription(description)
	description, limited := limitStringLength(description, maxLen)

	if limited {
		description += "…"
	}

	return description
}

type rssFeedRequest struct {
	URL             string            `yaml:"url"`
	Title           string            `yaml:"title"`
	HideCategories  bool              `yaml:"hide-categories"`
	HideDescription bool              `yaml:"hide-description"`
	Limit           int               `yaml:"limit"`
	ItemLinkPrefix  string            `yaml:"item-link-prefix"`
	Headers         map[string]string `yaml:"headers"`
	IsDetailed      bool              `yaml:"-"`
	IfModifiedSince time.Time         `yaml:"-"`
	Etag            string            `yaml:"-"`
}

type rssFeedItemList []rssFeedItem

type rssFeedResponse struct {
	URL          string
	Items        rssFeedItemList
	LastModified time.Time
	Etag         string
}

func (f rssFeedItemList) sortByNewest() rssFeedItemList {
	sort.Slice(f, func(i, j int) bool {
		return f[i].PublishedAt.After(f[j].PublishedAt)
	})

	return f
}

var feedParser = gofeed.NewParser()

func fetchItemsFromRSSFeedTask(request rssFeedRequest) (rssFeedResponse, error) {
	feedResponse := rssFeedResponse{URL: request.URL}

	req, err := http.NewRequest("GET", request.URL, nil)
	if err != nil {
		return feedResponse, err
	}

	req.Header.Add("User-Agent", fmt.Sprintf("Glance v%s", buildVersion))

	for key, value := range request.Headers {
		req.Header.Add(key, value)
	}

	if !request.IfModifiedSince.IsZero() {
		req.Header.Add("If-Modified-Since", request.IfModifiedSince.Format(http.TimeFormat))
	}

	if request.Etag != "" {
		req.Header.Add("If-None-Match", request.Etag)
	}

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return feedResponse, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return feedResponse, errNotModified
	}

	if resp.StatusCode != http.StatusOK {
		return feedResponse, fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, request.URL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return feedResponse, err
	}

	feed, err := feedParser.ParseString(string(body))
	if err != nil {
		return feedResponse, err
	}

	if request.Limit > 0 && len(feed.Items) > request.Limit {
		feed.Items = feed.Items[:request.Limit]
	}

	items := make([]rssFeedItem, 0, len(feed.Items))

	if lastModified := resp.Header.Get("Last-Modified"); lastModified != "" {
		if t, err := time.Parse(http.TimeFormat, lastModified); err == nil {
			feedResponse.LastModified = t
		}
	}

	if etag := resp.Header.Get("Etag"); etag != "" {
		feedResponse.Etag = etag
	}

	for i := range feed.Items {
		item := feed.Items[i]

		rssItem := rssFeedItem{
			ChannelURL: feed.Link,
		}

		if request.ItemLinkPrefix != "" {
			rssItem.Link = request.ItemLinkPrefix + item.Link
		} else if strings.HasPrefix(item.Link, "http://") || strings.HasPrefix(item.Link, "https://") {
			rssItem.Link = item.Link
		} else {
			parsedUrl, err := url.Parse(feed.Link)
			if err != nil {
				parsedUrl, err = url.Parse(request.URL)
			}

			if err == nil {
				var link string

				if len(item.Link) > 0 && item.Link[0] == '/' {
					link = item.Link
				} else {
					link = "/" + item.Link
				}

				rssItem.Link = parsedUrl.Scheme + "://" + parsedUrl.Host + link
			}
		}

		if item.Title != "" {
			rssItem.Title = html.UnescapeString(item.Title)
		} else {
			rssItem.Title = shortenFeedDescriptionLen(item.Description, 100)
		}

		if request.IsDetailed {
			if !request.HideDescription && item.Description != "" && item.Title != "" {
				rssItem.Description = shortenFeedDescriptionLen(item.Description, 200)
			}

			if !request.HideCategories {
				var categories = make([]string, 0, 6)

				for _, category := range item.Categories {
					if len(categories) == 6 {
						break
					}

					if len(category) == 0 || len(category) > 30 {
						continue
					}

					categories = append(categories, category)
				}

				rssItem.Categories = categories
			}
		}

		if request.Title != "" {
			rssItem.ChannelName = request.Title
		} else {
			rssItem.ChannelName = feed.Title
		}

		if item.Image != nil {
			rssItem.ImageURL = item.Image.URL
		} else if url := findThumbnailInItemExtensions(item); url != "" {
			rssItem.ImageURL = url
		} else if feed.Image != nil {
			if len(feed.Image.URL) > 0 && feed.Image.URL[0] == '/' {
				rssItem.ImageURL = strings.TrimRight(feed.Link, "/") + feed.Image.URL
			} else {
				rssItem.ImageURL = feed.Image.URL
			}
		}

		if item.PublishedParsed != nil {
			rssItem.PublishedAt = *item.PublishedParsed
		} else {
			rssItem.PublishedAt = time.Now()
		}

		items = append(items, rssItem)
	}

	feedResponse.Items = items
	return feedResponse, nil
}

func recursiveFindThumbnailInExtensions(extensions map[string][]gofeedext.Extension) string {
	for _, exts := range extensions {
		for _, ext := range exts {
			if ext.Name == "thumbnail" || ext.Name == "image" {
				if url, ok := ext.Attrs["url"]; ok {
					return url
				}
			}

			if ext.Children != nil {
				if url := recursiveFindThumbnailInExtensions(ext.Children); url != "" {
					return url
				}
			}
		}
	}

	return ""
}

func findThumbnailInItemExtensions(item *gofeed.Item) string {
	media, ok := item.Extensions["media"]

	if !ok {
		return ""
	}

	return recursiveFindThumbnailInExtensions(media)
}

func fetchItemsFromRSSFeeds(requests []rssFeedRequest, cachedFeeds map[string]cachedFeed) (rssFeedItemList, []rssFeedResponse, error) {
	job := newJob(fetchItemsFromRSSFeedTask, requests).withWorkers(30)
	feeds, errs, err := workerPoolDo(job)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", errNoContent, err)
	}

	failed := 0
	notModified := 0

	entries := make(rssFeedItemList, 0, len(feeds)*10)

	for i := range feeds {
		if errs[i] == nil {
			entries = append(entries, feeds[i].Items...)
		} else if errors.Is(errs[i], errNotModified) {
			notModified++
			entries = append(entries, cachedFeeds[feeds[i].URL].Items...)
			slog.Debug("Feed not modified", "url", requests[i].URL, "debug", errs[i])
		} else {
			failed++
			slog.Error("Failed to get RSS feed", "url", requests[i].URL, "error", errs[i])
		}
	}

	if failed == len(requests) {
		return nil, nil, errNoContent
	}

	if failed > 0 {
		return entries, feeds, fmt.Errorf("%w: missing %d RSS feeds", errPartialContent, failed)
	}

	return entries, feeds, nil
}
