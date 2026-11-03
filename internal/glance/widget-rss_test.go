package glance

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestRSSConditionalCachePreservesRequestOptions(t *testing.T) {
	const feed = `<?xml version="1.0"?>
<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/"><channel>
<title>Feed title</title><link>https://example.com</link><description>A feed</description>
<item><title>First</title><link>https://example.com/first</link>
<description>First description</description><category>News</category>
<media:thumbnail url="/first.jpg"/>
<pubDate>Mon, 02 Nov 2026 12:00:00 GMT</pubDate></item>
<item><title>Second</title><link>https://example.com/second</link>
<description>Second description</description><category>Updates</category>
<pubDate>Sun, 01 Nov 2026 12:00:00 GMT</pubDate></item>
</channel></rss>`
	for _, test := range []struct {
		name   string
		change func(*rssFeedRequest)
	}{
		{"title", func(r *rssFeedRequest) { r.Title = "Custom title" }},
		{"limit", func(r *rssFeedRequest) { r.Limit = 1 }},
		{"link prefix", func(r *rssFeedRequest) { r.ItemLinkPrefix = "https://proxy.example/?url=" }},
		{"thumbnail prefix", func(r *rssFeedRequest) { r.ThumbnailLinkPrefix = "https://images.example" }},
		{"hide description", func(r *rssFeedRequest) { r.HideDescription = true }},
		{"hide categories", func(r *rssFeedRequest) { r.HideCategories = true }},
		{"non-detailed", func(r *rssFeedRequest) { r.IsDetailed = false }},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests, notModified := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Header.Get("If-None-Match") == `"feed-v1"` {
					notModified++
					w.WriteHeader(http.StatusNotModified)
					return
				}
				w.Header().Set("ETag", `"feed-v1"`)
				w.Header().Set("Content-Type", "application/rss+xml")
				fmt.Fprint(w, feed)
			}))
			defer server.Close()
			newWidget := func() *rssWidget {
				widget := &rssWidget{}
				if err := widget.initialize(); err != nil {
					t.Fatal(err)
				}
				return widget
			}
			fetch := func(widget *rssWidget, request rssFeedRequest) []rssFeedItem {
				items, err := widget.fetchItemsFromFeedTask(request)
				if err != nil {
					t.Fatal(err)
				}
				return items
			}
			original := rssFeedRequest{URL: server.URL, IsDetailed: true}
			changed := original
			test.change(&changed)
			wantOriginal := fetch(newWidget(), original)
			wantChanged := fetch(newWidget(), changed)
			if reflect.DeepEqual(wantOriginal, wantChanged) {
				t.Fatal("fixture must exercise a request option that changes the result")
			}

			widget := newWidget()
			fetch(widget, changed)
			if got := fetch(widget, original); !reflect.DeepEqual(got, wantOriginal) {
				t.Errorf("cached original request = %+v, want %+v", got, wantOriginal)
			}
			if got := fetch(widget, changed); !reflect.DeepEqual(got, wantChanged) {
				t.Errorf("cached changed request = %+v, want %+v", got, wantChanged)
			}
			if requests != 5 || notModified != 2 {
				t.Errorf("requests = %d, 304 responses = %d; want 5 and 2", requests, notModified)
			}
		})
	}
}

func TestRSSConditionalCacheKeepsUndatedItems(t *testing.T) {
	for _, validator := range []struct {
		header, conditional, value string
	}{
		{"ETag", "If-None-Match", `"feed-v1"`},
		{"Last-Modified", "If-Modified-Since", "Mon, 02 Nov 2026 12:00:00 GMT"},
	} {
		t.Run(validator.header, func(t *testing.T) {
			notModified := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get(validator.conditional) == validator.value {
					notModified++
					w.WriteHeader(http.StatusNotModified)
					return
				}
				w.Header().Set(validator.header, validator.value)
				fmt.Fprint(w, `<rss version="2.0"><channel><title>Feed</title>
<link>https://example.com</link><description>Feed</description>
<item><title>Undated</title><link>https://example.com/item</link></item>
</channel></rss>`)
			}))
			defer server.Close()
			widget := &rssWidget{}
			if err := widget.initialize(); err != nil {
				t.Fatal(err)
			}
			request := rssFeedRequest{URL: server.URL}
			first, err := widget.fetchItemsFromFeedTask(request)
			if err != nil {
				t.Fatal(err)
			}
			if len(first) != 1 || first[0].PublishedAt.IsZero() {
				t.Fatalf("missing undated item or fallback time: %+v", first)
			}
			second, err := widget.fetchItemsFromFeedTask(request)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(first, second) || notModified != 1 {
				t.Errorf("304 changed undated items: first = %+v, second = %+v, 304 responses = %d", first, second, notModified)
			}
		})
	}
}
