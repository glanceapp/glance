package glance

import (
	"context"
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

type rssWidget struct {
	widgetBase       `yaml:",inline"`
	FeedRequests     []rssFeedRequest `yaml:"feeds"`
	Style            string           `yaml:"style"`
	ThumbnailHeight  float64          `yaml:"thumbnail-height"`
	CardHeight       float64          `yaml:"card-height"`
	Items            rssFeedItemList  `yaml:"-"`
	Limit            int              `yaml:"limit"`
	CollapseAfter    int              `yaml:"collapse-after"`
	SingleLineTitles bool             `yaml:"single-line-titles"`
	NoItemsMessage   string           `yaml:"-"`
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
	items, err := fetchItemsFromRSSFeeds(widget.FeedRequests)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(items) > widget.Limit {
		items = items[:widget.Limit]
	}

	widget.Items = items
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
	PublicURL       string            `yaml:"public-url"`
	Title           string            `yaml:"title"`
	HideCategories  bool              `yaml:"hide-categories"`
	HideDescription bool              `yaml:"hide-description"`
	ItemLinkPrefix  string            `yaml:"item-link-prefix"`
	Headers         map[string]string `yaml:"headers"`
	IsDetailed      bool              `yaml:"-"`
}

type rssFeedItemList []rssFeedItem

func (f rssFeedItemList) sortByNewest() rssFeedItemList {
	sort.Slice(f, func(i, j int) bool {
		return f[i].PublishedAt.After(f[j].PublishedAt)
	})

	return f
}

var feedParser = gofeed.NewParser()

func fetchItemsFromRSSFeedTask(request rssFeedRequest) ([]rssFeedItem, error) {
	req, err := http.NewRequest("GET", request.URL, nil)
	if err != nil {
		return nil, err
	}

	for key, value := range request.Headers {
		req.Header.Add(key, value)
	}

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, request.URL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	feed, err := feedParser.ParseString(string(body))
	if err != nil {
		return nil, err
	}

	items := make(rssFeedItemList, 0, len(feed.Items))

	for i := range feed.Items {
		item := feed.Items[i]

		rssItem := rssFeedItem{
			ChannelURL: feed.Link,
		}

		if request.PublicURL != "" {
			rssItem.ChannelURL = request.PublicURL
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

	return items, nil
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

func fetchItemsFromRSSFeeds(requests []rssFeedRequest) (rssFeedItemList, error) {
	job := newJob(fetchItemsFromRSSFeedTask, requests).withWorkers(30)
	feeds, errs, err := workerPoolDo(job)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errNoContent, err)
	}

	failed := 0

	entries := make(rssFeedItemList, 0, len(feeds)*10)

	for i := range feeds {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to get RSS feed", "url", requests[i].URL, "error", errs[i])
			continue
		}

		entries = append(entries, feeds[i]...)
	}

	if failed == len(requests) {
		return nil, errNoContent
	}

	entries.sortByNewest()

	if failed > 0 {
		return entries, fmt.Errorf("%w: missing %d RSS feeds", errPartialContent, failed)
	}

	return entries, nil
}
