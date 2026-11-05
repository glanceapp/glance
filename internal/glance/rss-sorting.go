package glance

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"time"
)

const (
	rssSortChronological = "chronological"
	rssSortRarity        = "rarity"
	rssSortTier          = "tier"

	defaultRarityBase     = 72.0
	defaultRarityExponent = 2.5
	defaultFeedGapHours   = 24.0
)

func validateRSSSort(sort string, base, exponent float64) error {
	switch sort {
	case "", rssSortChronological, rssSortRarity, rssSortTier:
	default:
		return fmt.Errorf("invalid sort %q, must be one of: chronological, rarity, tier", sort)
	}

	if base < 0 || exponent < 0 {
		return fmt.Errorf("rarity-base and rarity-exponent must not be negative")
	}

	return nil
}

func (f rssFeedItemList) sortByRarity(now time.Time, base, exponent float64) rssFeedItemList {
	if base == 0 {
		base = defaultRarityBase
	}
	if exponent == 0 {
		exponent = defaultRarityExponent
	}

	// Median hours between consecutive posts per feed, measured from the items
	// the feed returned. Median rather than mean so a single stale item in a
	// busy feed doesn't make it look rare.
	times := make(map[string][]time.Time)
	for _, item := range f {
		times[item.feedURL] = append(times[item.feedURL], item.PublishedAt)
	}

	gaps := make(map[string]float64, len(times))
	for url, ts := range times {
		gaps[url] = defaultFeedGapHours
		if len(ts) < 2 {
			continue
		}

		slices.SortFunc(ts, func(a, b time.Time) int { return a.Compare(b) })
		diffs := make([]float64, len(ts)-1)
		for i := range diffs {
			diffs[i] = ts[i+1].Sub(ts[i]).Hours()
		}
		slices.Sort(diffs)
		gaps[url] = diffs[len(diffs)/2]
	}

	// Lower effective age ranks first: age * (base/gap)^exponent, so rarely
	// posting feeds age slower than prolific ones.
	effectiveAge := func(item rssFeedItem) float64 {
		gap := max(gaps[item.feedURL], 0.1)

		multiplier := math.Pow(base/gap, exponent)
		multiplier = min(max(multiplier, 0.0001), 100)

		return now.Sub(item.PublishedAt).Hours() * multiplier
	}

	slices.SortStableFunc(f, func(a, b rssFeedItem) int {
		return cmp.Compare(effectiveAge(a), effectiveAge(b))
	})

	return f
}

// sortByTier orders by tier (lower first), then newest first. Feeds without a
// tier go last.
func (f rssFeedItemList) sortByTier(tiers map[string]int) rssFeedItemList {
	lowest := 0
	for _, tier := range tiers {
		lowest = max(lowest, tier)
	}
	lowest++

	tierOf := func(item rssFeedItem) int {
		if tier, ok := tiers[item.feedURL]; ok {
			return tier
		}
		return lowest
	}

	slices.SortStableFunc(f, func(a, b rssFeedItem) int {
		if ta, tb := tierOf(a), tierOf(b); ta != tb {
			return ta - tb
		}
		return b.PublishedAt.Compare(a.PublishedAt)
	})

	return f
}
