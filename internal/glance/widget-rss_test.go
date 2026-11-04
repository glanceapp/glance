package glance

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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

func TestRSSConditionalCacheSeparatesRequestHeaders(t *testing.T) {
	const modified = "Mon, 02 Nov 2026 12:00:00 GMT"
	for _, header := range []string{"Authorization", "Accept-Language"} {
		t.Run(header, func(t *testing.T) {
			notModified := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Vary", header)
				w.Header().Set("Last-Modified", modified)
				if r.Header.Get("If-Modified-Since") == modified {
					notModified++
					w.WriteHeader(http.StatusNotModified)
					return
				}
				fmt.Fprintf(w, `<rss version="2.0"><channel><title>Feed</title>
<link>https://example.com</link><description>Feed</description>
<item><title>%s</title><link>https://example.com/item</link></item>
</channel></rss>`, r.Header.Get(header))
			}))
			defer server.Close()
			widget := &rssWidget{}
			if err := widget.initialize(); err != nil {
				t.Fatal(err)
			}
			for index, variant := range []string{"first", "second", "first", "second"} {
				name := header
				if index >= 2 {
					name = strings.ToLower(header)
				}
				items, err := widget.fetchItemsFromFeedTask(rssFeedRequest{
					URL: server.URL, Headers: map[string]string{name: variant, "X-Feed": "fixture"},
				})
				if err != nil {
					t.Fatal(err)
				}
				if len(items) != 1 || items[0].Title != variant {
					t.Errorf("header %s=%s returned %+v", header, variant, items)
				}
			}
			if notModified != 2 {
				t.Errorf("304 responses = %d, want 2 (one repeat per representation)", notModified)
			}
		})
	}
}

func TestRSSConditionalCacheKeepsExplicitValidators(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for header, want := range map[string]string{
			"If-None-Match": `"configured"`, "If-Modified-Since": "Mon, 02 Nov 2026 12:00:00 GMT",
		} {
			if got := r.Header.Values(header); !reflect.DeepEqual(got, []string{want}) {
				t.Errorf("%s = %v, want only %q", header, got, want)
			}
		}
		w.Header().Set("ETag", `"server"`)
		w.Header().Set("Last-Modified", "Tue, 03 Nov 2026 12:00:00 GMT")
		fmt.Fprint(w, `<rss version="2.0"><channel><title>Feed</title>
<link>https://example.com</link><description>Feed</description></channel></rss>`)
	}))
	defer server.Close()
	widget := &rssWidget{}
	if err := widget.initialize(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		_, err := widget.fetchItemsFromFeedTask(rssFeedRequest{
			URL: server.URL, Headers: map[string]string{
				"if-none-match": `"configured"`, "if-modified-since": "Mon, 02 Nov 2026 12:00:00 GMT",
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
