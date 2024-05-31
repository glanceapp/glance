package assets

import (
	"fmt"
	"html/template"
	"math"
	"strconv"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var (
	PageTemplate                  = compileTemplate("page.html", "document.html", "page-style-overrides.gotmpl")
	PageContentTemplate           = compileTemplate("content.html")
	CalendarTemplate              = compileTemplate("calendar.html", "widget-base.html")
	ClockTemplate                 = compileTemplate("clock.html", "widget-base.html")
	BookmarksTemplate             = compileTemplate("bookmarks.html", "widget-base.html")
	IFrameTemplate                = compileTemplate("iframe.html", "widget-base.html")
	WeatherTemplate               = compileTemplate("weather.html", "widget-base.html")
	ForumPostsTemplate            = compileTemplate("forum-posts.html", "widget-base.html")
	RedditCardsHorizontalTemplate = compileTemplate("reddit-horizontal-cards.html", "widget-base.html")
	RedditCardsVerticalTemplate   = compileTemplate("reddit-vertical-cards.html", "widget-base.html")
	ReleasesTemplate              = compileTemplate("releases.html", "widget-base.html")
	ChangeDetectionTemplate       = compileTemplate("change-detection.html", "widget-base.html")
	VideosTemplate                = compileTemplate("videos.html", "widget-base.html", "video-card-contents.html")
	VideosGridTemplate            = compileTemplate("videos-grid.html", "widget-base.html", "video-card-contents.html")
	MarketsTemplate               = compileTemplate("markets.html", "widget-base.html")
	RSSListTemplate               = compileTemplate("rss-list.html", "widget-base.html")
	RSSDetailedListTemplate       = compileTemplate("rss-detailed-list.html", "widget-base.html")
	RSSHorizontalCardsTemplate    = compileTemplate("rss-horizontal-cards.html", "widget-base.html")
	RSSHorizontalCards2Template   = compileTemplate("rss-horizontal-cards-2.html", "widget-base.html")
	MonitorTemplate               = compileTemplate("monitor.html", "widget-base.html")
	TwitchGamesListTemplate       = compileTemplate("twitch-games-list.html", "widget-base.html")
	TwitchChannelsTemplate        = compileTemplate("twitch-channels.html", "widget-base.html")
	RepositoryTemplate            = compileTemplate("repository.html", "widget-base.html")
	SearchTemplate                = compileTemplate("search.html", "widget-base.html")
	ExtensionTemplate             = compileTemplate("extension.html", "widget-base.html")
)

var globalTemplateFunctions = template.FuncMap{
	"relativeTime":      relativeTimeSince,
	"formatViewerCount": formatViewerCount,
	"formatNumber":      intl.Sprint,
	"absInt": func(i int) int {
		return int(math.Abs(float64(i)))
	},
	"formatPrice": func(price float64) string {
		return intl.Sprintf("%.2f", price)
	},
	"formatTime": func(t time.Time) string {
		return t.Format("2006-01-02 15:04:05")
	},
	"shouldCollapse": func(i int, collapseAfter int) bool {
		if collapseAfter < -1 {
			return false
		}

		return i >= collapseAfter
	},
	"itemAnimationDelay": func(i int, collapseAfter int) string {
		return fmt.Sprintf("%dms", (i-collapseAfter)*30)
	},
	"dynamicRelativeTimeAttrs": func(t time.Time) template.HTMLAttr {
		return template.HTMLAttr(fmt.Sprintf(`data-dynamic-relative-time="%d"`, t.Unix()))
	},
}

func compileTemplate(primary string, dependencies ...string) *template.Template {
	t, err := template.New(primary).
		Funcs(globalTemplateFunctions).
		ParseFS(TemplateFS, append([]string{primary}, dependencies...)...)

	if err != nil {
		panic(err)
	}

	return t
}

var intl = message.NewPrinter(language.English)

func formatViewerCount(count int) string {
	if count < 1_000 {
		return strconv.Itoa(count)
	}

	if count < 10_000 {
		return fmt.Sprintf("%.1fk", float64(count)/1_000)
	}

	if count < 1_000_000 {
		return fmt.Sprintf("%dk", count/1_000)
	}

	return fmt.Sprintf("%.1fm", float64(count)/1_000_000)
}

func relativeTimeSince(t time.Time) string {
	delta := time.Since(t)

	if delta < time.Minute {
		return "1m"
	}
	if delta < time.Hour {
		return fmt.Sprintf("%dm", delta/time.Minute)
	}
	if delta < 24*time.Hour {
		return fmt.Sprintf("%dh", delta/time.Hour)
	}
	if delta < 30*24*time.Hour {
		return fmt.Sprintf("%dd", delta/(24*time.Hour))
	}
	if delta < 12*30*24*time.Hour {
		return fmt.Sprintf("%dmo", delta/(30*24*time.Hour))
	}

	return fmt.Sprintf("%dy", delta/(365*24*time.Hour))
}
