package feed

import (
	"math"
	"sort"
	"time"
)

type ForumPost struct {
	Title           string
	DiscussionUrl   string
	TargetUrl       string
	TargetUrlDomain string
	ThumbnailUrl    string
	CommentCount    int
	Score           int
	Engagement      float64
	TimePosted      time.Time
	Tags            []string
}

type ForumPosts []ForumPost

type Calendar struct {
	CurrentDay        int
	CurrentWeekNumber int
	CurrentMonthName  string
	CurrentYear       int
	Days              []int
}

type Weather struct {
	Temperature         int
	ApparentTemperature int
	WeatherCode         int
	CurrentColumn       int
	SunriseColumn       int
	SunsetColumn        int
	Columns             []weatherColumn
}

type AppRelease struct {
	Name         string
	Version      string
	NotesUrl     string
	TimeReleased time.Time
	Downvotes    int
}

type AppReleases []AppRelease

type Video struct {
	ThumbnailUrl string
	Title        string
	Url          string
	Author       string
	AuthorUrl    string
	TimePosted   time.Time
}

type Videos []Video

var currencyToSymbol = map[string]string{
	"USD": "$",
	"EUR": "€",
	"JPY": "¥",
	"CAD": "C$",
	"AUD": "A$",
	"GBP": "£",
	"CHF": "Fr",
	"NZD": "N$",
	"INR": "₹",
	"BRL": "R$",
	"RUB": "₽",
	"TRY": "₺",
	"ZAR": "R",
	"CNY": "¥",
	"KRW": "₩",
	"HKD": "HK$",
	"SGD": "S$",
	"SEK": "kr",
	"NOK": "kr",
	"DKK": "kr",
	"PLN": "zł",
	"PHP": "₱",
}

type MarketRequest struct {
	Name       string `yaml:"name"`
	Symbol     string `yaml:"symbol"`
	ChartLink  string `yaml:"chart-link"`
	SymbolLink string `yaml:"symbol-link"`
}

type Market struct {
	MarketRequest
	Currency       string  `yaml:"-"`
	Price          float64 `yaml:"-"`
	PercentChange  float64 `yaml:"-"`
	SvgChartPoints string  `yaml:"-"`
}

type Markets []Market

func (t Markets) SortByAbsChange() {
	sort.Slice(t, func(i, j int) bool {
		return math.Abs(t[i].PercentChange) > math.Abs(t[j].PercentChange)
	})
}

var weatherCodeTable = map[int]string{
	0:  "Clear Sky",
	1:  "Mainly Clear",
	2:  "Partly Cloudy",
	3:  "Overcast",
	45: "Fog",
	48: "Rime Fog",
	51: "Drizzle",
	53: "Drizzle",
	55: "Drizzle",
	56: "Drizzle",
	57: "Drizzle",
	61: "Rain",
	63: "Moderate Rain",
	65: "Heavy Rain",
	66: "Freezing Rain",
	67: "Freezing Rain",
	71: "Snow",
	73: "Moderate Snow",
	75: "Heavy Snow",
	77: "Snow Grains",
	80: "Rain",
	81: "Moderate Rain",
	82: "Heavy Rain",
	85: "Snow",
	86: "Snow",
	95: "Thunderstorm",
	96: "Thunderstorm",
	99: "Thunderstorm",
}

func (w *Weather) WeatherCodeAsString() string {
	if weatherCode, ok := weatherCodeTable[w.WeatherCode]; ok {
		return weatherCode
	}

	return ""
}

const depreciatePostsOlderThanHours = 7
const maxDepreciation = 0.9
const maxDepreciationAfterHours = 24

func (p ForumPosts) CalculateEngagement() {
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

func (p ForumPosts) SortByEngagement() {
	sort.Slice(p, func(i, j int) bool {
		return p[i].Engagement > p[j].Engagement
	})
}

func (s *ForumPost) HasTargetUrl() bool {
	return s.TargetUrl != ""
}

func (p ForumPosts) FilterPostedBefore(postedBefore time.Duration) []ForumPost {
	recent := make([]ForumPost, 0, len(p))

	for i := range p {
		if time.Since(p[i].TimePosted) < postedBefore {
			recent = append(recent, p[i])
		}
	}

	return recent
}

func (r AppReleases) SortByNewest() AppReleases {
	sort.Slice(r, func(i, j int) bool {
		return r[i].TimeReleased.After(r[j].TimeReleased)
	})

	return r
}

func (v Videos) SortByNewest() Videos {
	sort.Slice(v, func(i, j int) bool {
		return v[i].TimePosted.After(v[j].TimePosted)
	})

	return v
}
