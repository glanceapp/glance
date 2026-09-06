package glance

import "testing"

// A feed whose own <link> is relative cannot be the base for absolutizing its
// item links. url.Parse accepts it without an error and leaves the scheme and
// the host empty, so the link is built as ":///path", which html/template will
// not render in an href and replaces with #ZgotmplZ. The feed URL is always
// absolute and stands in for those feeds.
func TestResolveItemLink(t *testing.T) {
	const feedURL = "https://samwho.dev/rss.xml"

	tests := []struct {
		name     string
		itemLink string
		feedLink string
		want     string
	}{
		{"absolute item link is left alone", "https://example.com/post", "https://example.com", "https://example.com/post"},
		{"http is absolute too", "http://example.com/post", "https://example.com", "http://example.com/post"},
		{"relative item link uses the feed link", "/post", "https://example.com", "https://example.com/post"},
		{"item link without a leading slash gets one", "post", "https://example.com", "https://example.com/post"},
		{"path and query of the feed link are dropped", "/post", "https://example.com/blog?x=1", "https://example.com/post"},
		// The feed reported in #962 is an Atom feed with <link href="/" /> and
		// entries such as /2025/, which is where ":///2025/" came from.
		{"relative feed link falls back to the feed url", "/2025/", "/", "https://samwho.dev/2025/"},
		{"empty feed link falls back to the feed url", "/post", "", "https://samwho.dev/post"},
		{"schemeless feed link falls back to the feed url", "/post", "example.com", "https://samwho.dev/post"},
		{"a feed link with no host falls back to the feed url", "/post", "https:///post", "https://samwho.dev/post"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveItemLink(tt.itemLink, tt.feedLink, feedURL)
			if got != tt.want {
				t.Errorf("resolveItemLink(%q, %q, %q) = %q, want %q", tt.itemLink, tt.feedLink, feedURL, got, tt.want)
			}
		})
	}
}

// With no absolute base anywhere the link is dropped rather than built into
// something html/template will refuse to render.
func TestResolveItemLinkWithoutAnyAbsoluteBase(t *testing.T) {
	if got := resolveItemLink("/post", "/", "/rss.xml"); got != "" {
		t.Errorf("resolveItemLink() = %q, want an empty string", got)
	}
}
