package feed

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
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
}

type FreshRssFeedsGroups struct {
	Group_id int
	Feed_ids string
}

type FreshRssFeed struct {
	Id                   int
	Favicon_id           int
	Title                string
	Url                  string
	Site_url             string
	Is_spark             int
	Last_updated_on_time int
}

type FreshRSSFeedsAPI struct {
	Api_version            uint
	Auth                   uint
	Last_refreshed_on_time int
	Feeds                  []FreshRssFeed
	Feeds_groups           []FreshRssFeedsGroups
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

type RSSFeedRequest struct {
	Url             string `yaml:"url"`
	Title           string `yaml:"title"`
	HideCategories  bool   `yaml:"hide-categories"`
	HideDescription bool   `yaml:"hide-description"`
	ItemLinkPrefix  string `yaml:"item-link-prefix"`
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

				if item.Link[0] == '/' {
					link = item.Link
				} else {
					link = "/" + item.Link
				}

				rssItem.Link = parsedUrl.Scheme + "://" + parsedUrl.Host + link
			}
		}

		if !request.HideDescription && item.Description != "" {
			description, _ := limitStringLength(item.Description, 1000)
			description = sanitizeFeedDescription(description)
			description, limited := limitStringLength(description, 200)

			if limited {
				description += "…"
			}

			rssItem.Description = description
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

func GetItemsFromFreshRssFeeds(freshrssUrl string, freshrssUser string, freshrsspass string) (RSSFeedItems, error) {
	var p FreshRSSFeedsAPI
	var feedReqs []RSSFeedRequest
	var param = url.Values{}

	user_credentials := []byte(fmt.Sprintf("%v:%v", freshrssUser, freshrsspass))
	api_key := fmt.Sprintf("%x", md5.Sum(user_credentials))

	param.Set("api_key", api_key)
	param.Set("feeds", "")
	var payload = bytes.NewBufferString(param.Encode())

	requestURL := fmt.Sprintf("%v/api/fever.php?api", freshrssUrl)
	req, err := http.NewRequest(http.MethodPost, requestURL, payload)

	if err != nil {
		return nil, fmt.Errorf("could not create freshRss request: %v ", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil || res.StatusCode != 200 {
		return nil, fmt.Errorf("could not connect to freshRss instance: %v", err)
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read freshRss response body: %v", err)
	}

	errr := json.Unmarshal(resBody, &p)
	if errr != nil {
		return nil, fmt.Errorf("could not unmarshal freshrss response body: %v", errr)
	}

	for i := range p.Feeds {
		var feedReq RSSFeedRequest
		feedReq.Url = p.Feeds[i].Url
		feedReq.Title = p.Feeds[i].Title
		feedReqs = append(feedReqs, feedReq)
	}

	return GetItemsFromRSSFeeds(feedReqs)
}
