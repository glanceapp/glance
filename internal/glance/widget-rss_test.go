package glance

import (
	"testing"
	"time"
)

func TestParseRSSEndOfDayTime(t *testing.T) {
	tests := []struct {
		name string
		date string
		want time.Time
	}{
		{
			name: "RSS pubDate in GMT",
			date: "Sun, 07 Dec 2025 24:00:00 GMT",
			want: time.Date(2025, time.December, 8, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "RSS pubDate with a numeric offset",
			date: "Sun, 07 Dec 2025 24:00:00 +0200",
			want: time.Date(2025, time.December, 8, 0, 0, 0, 0, time.FixedZone("", 2*60*60)),
		},
		{
			name: "Atom timestamp",
			date: "2025-12-07T24:00:00Z",
			want: time.Date(2025, time.December, 8, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "rolls over the end of a year",
			date: "Wed, 31 Dec 2025 24:00:00 GMT",
			want: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "no zone at all",
			date: "2025-12-07T24:00:00",
			want: time.Date(2025, time.December, 8, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := parseRSSEndOfDayTime(test.date)
			if !ok {
				t.Fatalf("parseRSSEndOfDayTime(%q) did not recognize the date", test.date)
			}

			if !got.Equal(test.want) {
				t.Errorf("parseRSSEndOfDayTime(%q) = %v, want %v", test.date, got, test.want)
			}
		})
	}
}

func TestParseRSSEndOfDayTimeLeavesOtherDatesAlone(t *testing.T) {
	// None of these spell an hour of 24, so the date has to be reported as
	// unrecognized rather than quietly moved on by a day.
	dates := []string{
		"Sun, 07 Dec 2025 12:24:00 GMT",
		"Sun, 07 Dec 2025 00:00:24 GMT",
		"Wed, 24 Dec 2025 09:00:00 GMT",
		"2024-12-07T09:00:00Z",
		"",
		"not a date at all",
	}

	for _, date := range dates {
		if got, ok := parseRSSEndOfDayTime(date); ok {
			t.Errorf("parseRSSEndOfDayTime(%q) = %v, want it to be unrecognized", date, got)
		}
	}
}

// A feed that writes midnight as 24:00:00 is the case reported in #900: gofeed
// cannot parse the date, and the item used to fall back to the current time, so
// every post in the feed showed up as published today.
func TestRSSItemPublishedAtWithEndOfDayHour(t *testing.T) {
	const feed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Example</title>
    <item>
      <title>A post</title>
      <link>https://example.com/a-post</link>
      <pubDate>Sun, 07 Dec 2025 24:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`

	parsed, err := feedParser.ParseString(feed)
	if err != nil {
		t.Fatalf("parsing the feed: %v", err)
	}

	item := parsed.Items[0]
	if item.PublishedParsed != nil {
		t.Fatalf("expected the feed parser to leave the date unparsed, got %v", *item.PublishedParsed)
	}

	want := time.Date(2025, time.December, 8, 0, 0, 0, 0, time.UTC)
	if got := rssItemPublishedAt(item); !got.Equal(want) {
		t.Errorf("rssItemPublishedAt() = %v, want %v", got, want)
	}
}

func TestRSSItemPublishedAtPrefersTheParsedDate(t *testing.T) {
	const feed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Example</title>
    <item>
      <title>A post</title>
      <link>https://example.com/a-post</link>
      <pubDate>Sun, 07 Dec 2025 09:30:00 GMT</pubDate>
    </item>
  </channel>
</rss>`

	parsed, err := feedParser.ParseString(feed)
	if err != nil {
		t.Fatalf("parsing the feed: %v", err)
	}

	want := time.Date(2025, time.December, 7, 9, 30, 0, 0, time.UTC)
	if got := rssItemPublishedAt(parsed.Items[0]); !got.Equal(want) {
		t.Errorf("rssItemPublishedAt() = %v, want %v", got, want)
	}
}
