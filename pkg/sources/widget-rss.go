package sources

import (
	"context"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	gofeedext "github.com/mmcdole/gofeed/extensions"
)

var feedParser = gofeed.NewParser()

type rssSource struct {
	sourceBase       `yaml:",inline"`
	FeedRequests     []rssFeedRequest `yaml:"feeds"`
	Style            string           `yaml:"style"`
	ThumbnailHeight  float64          `yaml:"thumbnail-height"`
	CardHeight       float64          `yaml:"card-height"`
	Limit            int              `yaml:"limit"`
	CollapseAfter    int              `yaml:"collapse-after"`
	SingleLineTitles bool             `yaml:"single-line-titles"`
	PreserveOrder    bool             `yaml:"preserve-order"`

	Items          rssFeedItemList `yaml:"-"`
	NoItemsMessage string          `yaml:"-"`

	cachedFeedsMutex sync.Mutex
	cachedFeeds      map[string]*cachedRSSFeed `yaml:"-"`
}

func (s *rssSource) initialize() error {
	s.withTitle("RSS Feed").withCacheDuration(2 * time.Hour)

	if s.Limit <= 0 {
		s.Limit = 25
	}

	if s.CollapseAfter == 0 || s.CollapseAfter < -1 {
		s.CollapseAfter = 5
	}

	if s.ThumbnailHeight < 0 {
		s.ThumbnailHeight = 0
	}

	if s.CardHeight < 0 {
		s.CardHeight = 0
	}

	if s.Style == "detailed-list" {
		for i := range s.FeedRequests {
			s.FeedRequests[i].IsDetailed = true
		}
	}

	s.NoItemsMessage = "No items were returned from the feeds."
	s.cachedFeeds = make(map[string]*cachedRSSFeed)

	return nil
}

func (s *rssSource) update(ctx context.Context) {
	items, err := s.fetchItemsFromFeeds()

	if !s.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if !s.PreserveOrder {
		items.sortByNewest()
	}

	if len(items) > s.Limit {
		items = items[:s.Limit]
	}

	s.Items = items
}

func (s *rssSource) Feed() rssFeedItemList {
	return s.Items
}

type cachedRSSFeed struct {
	etag         string
	lastModified string
	items        []rssFeedItem
}

type rssFeedItem struct {
	ID          string
	Summary     string
	MatchScore  int
	ChannelName string
	ChannelURL  string
	Title       string
	Link        string
	ImageURL    string
	Categories  []string
	Description string
	PublishedAt time.Time
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
}

type rssFeedItemList []rssFeedItem

func (f rssFeedItemList) sortByNewest() rssFeedItemList {
	sort.Slice(f, func(i, j int) bool {
		return f[i].PublishedAt.After(f[j].PublishedAt)
	})

	return f
}

func (s *rssSource) fetchItemsFromFeeds() (rssFeedItemList, error) {
	requests := s.FeedRequests

	job := newJob(s.fetchItemsFromFeedTask, requests).withWorkers(30)
	feeds, errs, err := workerPoolDo(job)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errNoContent, err)
	}

	failed := 0
	entries := make(rssFeedItemList, 0, len(feeds)*10)
	seen := make(map[string]struct{})

	for i := range feeds {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to get RSS feed", "url", requests[i].URL, "error", errs[i])
			continue
		}

		for _, item := range feeds[i] {
			if _, exists := seen[item.Link]; exists {
				continue
			}
			entries = append(entries, item)
			seen[item.Link] = struct{}{}
		}
	}

	if failed == len(requests) {
		return nil, errNoContent
	}

	if failed > 0 {
		return entries, fmt.Errorf("%w: missing %d RSS feeds", errPartialContent, failed)
	}

	return entries, nil
}

func (s *rssSource) fetchItemsFromFeedTask(request rssFeedRequest) ([]rssFeedItem, error) {
	req, err := http.NewRequest("GET", request.URL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", pulseUserAgentString)

	s.cachedFeedsMutex.Lock()
	cache, isCached := s.cachedFeeds[request.URL]
	if isCached {
		if cache.etag != "" {
			req.Header.Add("If-None-Match", cache.etag)
		}
		if cache.lastModified != "" {
			req.Header.Add("If-Modified-Since", cache.lastModified)
		}
	}
	s.cachedFeedsMutex.Unlock()

	for key, value := range request.Headers {
		req.Header.Set(key, value)
	}

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified && isCached {
		return cache.items, nil
	}

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

	if request.Limit > 0 && len(feed.Items) > request.Limit {
		feed.Items = feed.Items[:request.Limit]
	}

	items := make(rssFeedItemList, 0, len(feed.Items))

	for i := range feed.Items {
		item := feed.Items[i]

		rssItem := rssFeedItem{
			ID:         item.GUID,
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

	if resp.Header.Get("ETag") != "" || resp.Header.Get("Last-Modified") != "" {
		s.cachedFeedsMutex.Lock()
		s.cachedFeeds[request.URL] = &cachedRSSFeed{
			etag:         resp.Header.Get("ETag"),
			lastModified: resp.Header.Get("Last-Modified"),
			items:        items,
		}
		s.cachedFeedsMutex.Unlock()
	}

	return items, nil
}

func findThumbnailInItemExtensions(item *gofeed.Item) string {
	media, ok := item.Extensions["media"]

	if !ok {
		return ""
	}

	return recursiveFindThumbnailInExtensions(media)
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

var htmlTagsWithAttributesPattern = regexp.MustCompile(`<\/?[a-zA-Z0-9-]+ *(?:[a-zA-Z-]+=(?:"|').*?(?:"|') ?)* *\/?>`)
var sequentialWhitespacePattern = regexp.MustCompile(`\s+`)

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
