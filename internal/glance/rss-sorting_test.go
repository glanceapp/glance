package glance

import (
	"testing"
	"time"
)

var sortNow = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func ago(d time.Duration) time.Time { return sortNow.Add(-d) }

func feeds(items rssFeedItemList) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.feedURL
	}
	return out
}

func TestValidateRSSSort(t *testing.T) {
	for _, tc := range []struct {
		sort    string
		base    float64
		wantErr bool
	}{
		{"", 0, false},
		{"rarity", 10, false},
		{"tier", 0, false},
		{"bogus", 0, true},
		{"rarity", -1, true},
	} {
		if err := validateRSSSort(tc.sort, tc.base, 0); (err != nil) != tc.wantErr {
			t.Errorf("sort=%q base=%v: err=%v, wantErr=%v", tc.sort, tc.base, err, tc.wantErr)
		}
	}
}

func TestSortByTier(t *testing.T) {
	items := rssFeedItemList{
		{feedURL: "b", PublishedAt: ago(1 * time.Hour)},
		{feedURL: "none", PublishedAt: ago(0)},
		{feedURL: "a", PublishedAt: ago(5 * time.Hour)},
		{feedURL: "a", PublishedAt: ago(2 * time.Hour)},
	}

	got := feeds(items.sortByTier(map[string]int{"a": 0, "b": 1}))
	want := []string{"a", "a", "b", "none"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if !items[0].PublishedAt.After(items[1].PublishedAt) {
		t.Error("items within a tier should be newest first")
	}
}

func TestSortByRarity(t *testing.T) {
	// "busy" posts hourly, "rare" every 10 days. A 3 day old rare post should
	// outrank a 2 hour old busy one.
	var items rssFeedItemList
	for i := 0; i < 5; i++ {
		items = append(items, rssFeedItem{feedURL: "busy", PublishedAt: ago(time.Duration(2+i) * time.Hour)})
	}
	items = append(items,
		rssFeedItem{feedURL: "rare", PublishedAt: ago(3 * 24 * time.Hour)},
		rssFeedItem{feedURL: "rare", PublishedAt: ago(13 * 24 * time.Hour)},
	)

	got := items.sortByRarity(sortNow, 0, 0)
	if got[0].feedURL != "rare" {
		t.Errorf("rare feed item should rank first, got %v", feeds(got))
	}
}

func TestSortByRaritySingleItemFeedUsesDefaultGap(t *testing.T) {
	items := rssFeedItemList{
		{feedURL: "a", PublishedAt: ago(5 * time.Hour)},
		{feedURL: "b", PublishedAt: ago(1 * time.Hour)},
	}

	got := items.sortByRarity(sortNow, 0, 0)
	if got[0].feedURL != "b" {
		t.Errorf("equal gaps should fall back to newest first, got %v", feeds(got))
	}
}

func TestSortByRarityIgnoresStaleOutlier(t *testing.T) {
	// "busy" posts every 10 minutes but includes one 30 day old item. A mean
	// gap would make it look rare; the median keeps it busy.
	var items rssFeedItemList
	for i := 0; i < 6; i++ {
		items = append(items, rssFeedItem{feedURL: "busy", PublishedAt: ago(time.Duration(10*(i+1)) * time.Minute)})
	}
	items = append(items,
		rssFeedItem{feedURL: "busy", PublishedAt: ago(30 * 24 * time.Hour)},
		rssFeedItem{feedURL: "rare", PublishedAt: ago(3 * 24 * time.Hour)},
		rssFeedItem{feedURL: "rare", PublishedAt: ago(13 * 24 * time.Hour)},
	)

	got := items.sortByRarity(sortNow, 0, 0)
	if got[0].feedURL != "rare" {
		t.Errorf("rare feed item should rank first, got %v", feeds(got))
	}
}
