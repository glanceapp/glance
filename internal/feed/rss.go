package feed

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/mmcdole/gofeed"
)

type RSSFeedItem struct {
	ChannelName string
	ChannelURL  string
	Title       string
	Link        string
	ImageURL    string
	PublishedAt time.Time
}

type RSSFeedRequest struct {
	Url   string `yaml:"url"`
	Title string `yaml:"title"`
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
			Title:      item.Title,
			Link:       item.Link,
		}

		if request.Title != "" {
			rssItem.ChannelName = request.Title
		} else {
			rssItem.ChannelName = feed.Title
		}

		if item.Image != nil {
			rssItem.ImageURL = item.Image.URL
		} else if feed.Image != nil {
			rssItem.ImageURL = feed.Image.URL
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

	if len(entries) == 0 {
		return nil, ErrNoContent
	}

	entries.SortByNewest()

	if failed > 0 {
		return entries, fmt.Errorf("%w: missing %d RSS feeds", ErrPartialContent, failed)
	}

	return entries, nil
}
