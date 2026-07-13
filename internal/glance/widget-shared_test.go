package glance

import (
	"math"
	"testing"
	"time"
)

// Every post is given an identical comment count and score so that each post's
// pre-depreciation engagement is exactly 1.0. That isolates the time-based
// depreciation applied by calculateEngagement, which is the only thing that
// should differentiate the posts.
func TestCalculateEngagementTimeDepreciation(t *testing.T) {
	now := time.Now()

	posts := forumPostList{
		{Title: "fresh", CommentCount: 100, Score: 100, TimePosted: now.Add(-1 * time.Hour)},
		{Title: "recent", CommentCount: 100, Score: 100, TimePosted: now.Add(-8 * time.Hour)},
		{Title: "old", CommentCount: 100, Score: 100, TimePosted: now.Add(-50 * time.Hour)},
	}

	posts.calculateEngagement()

	fresh := posts[0].Engagement
	recent := posts[1].Engagement
	old := posts[2].Engagement

	// A post younger than depreciatePostsOlderThanHours keeps its full engagement.
	if math.Abs(fresh-1.0) > 1e-6 {
		t.Errorf("fresh post engagement = %v, want 1.0", fresh)
	}

	// Depreciation must be gradual: an 8h-old post is only ~1h past the 7h
	// threshold, so it should lose only a few percent of its engagement rather
	// than being immediately slammed to the maximum depreciation.
	if recent < 0.9 {
		t.Errorf("8h-old post engagement = %v, want > 0.9 (gradual depreciation)", recent)
	}

	// Depreciation is capped at maxDepreciation (0.9), so engagement is floored
	// at 1-0.9 = 0.1 and must never go negative no matter how old the post is.
	if old < 0 {
		t.Errorf("50h-old post engagement = %v, want >= 0 (depreciation must be capped, never negative)", old)
	}
	if math.Abs(old-(1.0-maxDepreciation)) > 1e-6 {
		t.Errorf("50h-old post engagement = %v, want %v (floored at 1-maxDepreciation)", old, 1.0-maxDepreciation)
	}

	// Engagement must decrease monotonically with age.
	if !(fresh >= recent && recent > old) {
		t.Errorf("engagement not decreasing with age: fresh=%v recent=%v old=%v", fresh, recent, old)
	}
}
