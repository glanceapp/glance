package sources

import (
	"math"
	"sort"
	"time"
)

type forumPost struct {
	ID          string
	title       string
	Description string
	// MatchSummary is the LLM generated rationale for why this is a good match for the filter query
	MatchSummary string
	// MatchScore is the LLM generated score indicating how well this post matches the query
	MatchScore      int
	DiscussionUrl   string
	TargetUrl       string
	TargetUrlDomain string
	ThumbnailUrl    string
	CommentCount    int
	Score           int
	Engagement      float64
	TimePosted      time.Time
	Tags            []string
	IsCrosspost     bool
}

func (f forumPost) UID() string {
	return f.ID
}

func (f forumPost) Title() string {
	return f.title
}

func (f forumPost) Body() string {
	return f.Description
}

func (f forumPost) URL() string {
	return f.TargetUrl
}

func (f forumPost) ImageURL() string {
	return f.ThumbnailUrl
}

func (f forumPost) CreatedAt() time.Time {
	return f.TimePosted
}

type forumPostList []forumPost

const depreciatePostsOlderThanHours = 7
const maxDepreciation = 0.9
const maxDepreciationAfterHours = 24

func (p forumPostList) calculateEngagement() {
	var totalComments int
	var totalScore int

	for i := range p {
		totalComments += p[i].CommentCount
		totalScore += p[i].Score
	}

	numberOfPosts := float64(len(p))
	averageComments := float64(totalComments) / numberOfPosts
	averageScore := float64(totalScore) / numberOfPosts

	for i := range p {
		p[i].Engagement = (float64(p[i].CommentCount)/averageComments + float64(p[i].Score)/averageScore) / 2

		elapsed := time.Since(p[i].TimePosted)

		if elapsed < time.Hour*depreciatePostsOlderThanHours {
			continue
		}

		p[i].Engagement *= 1.0 - (math.Max(elapsed.Hours()-depreciatePostsOlderThanHours, maxDepreciationAfterHours)/maxDepreciationAfterHours)*maxDepreciation
	}
}

func (p forumPostList) sortByEngagement() {
	sort.Slice(p, func(i, j int) bool {
		return p[i].Engagement > p[j].Engagement
	})
}
