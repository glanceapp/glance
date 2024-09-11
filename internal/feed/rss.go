package feed

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	gofeedext "github.com/mmcdole/gofeed/extensions"
)

type RSSFeedItem struct {
	ChannelName string
	ChannelURL  string
	Title       string
	Link        string
	ImageURL    string
	Categories  []string
	Description string
	PublishedAt time.Time
	SourceName  string
	SourceURL   string
}

// doesn't cover all cases but works the vast majority of the time
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

type RSSFeedRequest struct {
	Url             string `yaml:"url"`
	Title           string `yaml:"title"`
	HideCategories  bool   `yaml:"hide-categories"`
	HideDescription bool   `yaml:"hide-description"`
	ItemLinkPrefix  string `yaml:"item-link-prefix"`
	IsDetailed      bool   `yaml:"-"`
	HideTitle       bool   `yaml:"hide-title"`
	ShowSource      bool   `yaml:"show-domain-source"`
}

type RSSFeedItems []RSSFeedItem

func (f RSSFeedItems) SortByNewest() RSSFeedItems {
	sort.Slice(f, func(i, j int) bool {
		return f[i].PublishedAt.After(f[j].PublishedAt)
	})

	return f
}

var feedParser = gofeed.NewParser()

func getItemsFromRSSFeedTask(request RSSFeedRequest) ([]RSSFeedItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	feed, err := feedParser.ParseURLWithContext(request.Url, ctx)

	if err != nil {
		return nil, err
	}

	items := make(RSSFeedItems, 0, len(feed.Items))

	for i := range feed.Items {
		item := feed.Items[i]

		rssItem := RSSFeedItem{
			ChannelURL: feed.Link,
		}

		if request.ItemLinkPrefix != "" {
			rssItem.Link = request.ItemLinkPrefix + item.Link
		} else if strings.HasPrefix(item.Link, "http://") || strings.HasPrefix(item.Link, "https://") {
			rssItem.Link = item.Link
		} else {
			parsedUrl, err := url.Parse(feed.Link)

			if err != nil {
				parsedUrl, err = url.Parse(request.Url)
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
			rssItem.Title = item.Title
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

		if request.ShowSource {
			parsedUrl, err := url.Parse(rssItem.Link)
			if err != nil {
				return nil, err
			}
			rssItem.SourceName = parsedUrl.Host
			rssItem.SourceURL = parsedUrl.Scheme + "://" + parsedUrl.Host
		}

		if request.HideTitle {
			rssItem.ChannelName = ""
		} else if request.Title != "" {
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

func GetItemsFromRSSFeeds(requests []RSSFeedRequest) (RSSFeedItems, error) {
	job := newJob(getItemsFromRSSFeedTask, requests).withWorkers(10)
	feeds, errs, err := workerPoolDo(job)

	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoContent, err)
	}

	failed := 0

	entries := make(RSSFeedItems, 0, len(feeds)*10)

	for i := range feeds {
		if errs[i] != nil {
			failed++
			slog.Error("failed to get rss feed", "error", errs[i], "url", requests[i].Url)
			continue
		}

		entries = append(entries, feeds[i]...)
	}

	if failed == len(requests) {
		return nil, ErrNoContent
	}

	entries.SortByNewest()

	if failed > 0 {
		return entries, fmt.Errorf("%w: missing %d RSS feeds", ErrPartialContent, failed)
	}

	return entries, nil
}
