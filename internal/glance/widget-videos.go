package glance

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var (
	videosWidgetTemplate     = mustParseTemplate("videos.html", "widget-base.html", "video-card-contents.html")
	videosWidgetGridTemplate = mustParseTemplate("videos-grid.html", "widget-base.html", "video-card-contents.html")
)

type videosWidget struct {
	widgetBase        `yaml:",inline"`
	Videos            videoList `yaml:"-"`
	VideoUrlTemplate  string    `yaml:"video-url-template"`
	Style             string    `yaml:"style"`
	CollapseAfterRows int       `yaml:"collapse-after-rows"`
	Channels          []string  `yaml:"channels"`
	Limit             int       `yaml:"limit"`
	IncludeShorts     bool      `yaml:"include-shorts"`
}

func (widget *videosWidget) initialize() error {
	widget.withTitle("Videos").withCacheDuration(time.Hour)

	if widget.Limit <= 0 {
		widget.Limit = 25
	}

	if widget.CollapseAfterRows == 0 || widget.CollapseAfterRows < -1 {
		widget.CollapseAfterRows = 4
	}

	return nil
}

func (widget *videosWidget) update(ctx context.Context) {
	videos, err := FetchYoutubeChannelUploads(widget.Channels, widget.VideoUrlTemplate, widget.IncludeShorts)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(videos) > widget.Limit {
		videos = videos[:widget.Limit]
	}

	widget.Videos = videos
}

func (widget *videosWidget) Render() template.HTML {
	if widget.Style == "grid-cards" {
		return widget.renderTemplate(widget, videosWidgetGridTemplate)
	}

	return widget.renderTemplate(widget, videosWidgetTemplate)
}

type youtubeFeedResponseXml struct {
	Channel     string `xml:"author>name"`
	ChannelLink string `xml:"author>uri"`
	Videos      []struct {
		Title     string `xml:"title"`
		Published string `xml:"published"`
		Link      struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`

		Group struct {
			Thumbnail struct {
				Url string `xml:"url,attr"`
			} `xml:"http://search.yahoo.com/mrss/ thumbnail"`
		} `xml:"http://search.yahoo.com/mrss/ group"`
	} `xml:"entry"`
}

func parseYoutubeFeedTime(t string) time.Time {
	parsedTime, err := time.Parse("2006-01-02T15:04:05-07:00", t)
	if err != nil {
		return time.Now()
	}

	return parsedTime
}

type video struct {
	ThumbnailUrl string
	Title        string
	Url          string
	Author       string
	AuthorUrl    string
	TimePosted   time.Time
}

type videoList []video

func (v videoList) sortByNewest() videoList {
	sort.Slice(v, func(i, j int) bool {
		return v[i].TimePosted.After(v[j].TimePosted)
	})

	return v
}

func FetchYoutubeChannelUploads(channelIds []string, videoUrlTemplate string, includeShorts bool) (videoList, error) {
	requests := make([]*http.Request, 0, len(channelIds))

	for i := range channelIds {
		var feedUrl string
		if !includeShorts && strings.HasPrefix(channelIds[i], "UC") {
			playlistId := strings.Replace(channelIds[i], "UC", "UULF", 1)
			feedUrl = "https://www.youtube.com/feeds/videos.xml?playlist_id=" + playlistId
		} else {
			feedUrl = "https://www.youtube.com/feeds/videos.xml?channel_id=" + channelIds[i]
		}

		request, _ := http.NewRequest("GET", feedUrl, nil)
		requests = append(requests, request)
	}

	job := newJob(decodeXmlFromRequestTask[youtubeFeedResponseXml](defaultClient), requests).withWorkers(30)

	responses, errs, err := workerPoolDo(job)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errNoContent, err)
	}

	videos := make(videoList, 0, len(channelIds)*15)

	var failed int

	for i := range responses {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to fetch youtube feed", "channel", channelIds[i], "error", errs[i])
			continue
		}

		response := responses[i]

		for j := range response.Videos {
			v := &response.Videos[j]
			var videoUrl string

			if videoUrlTemplate == "" {
				videoUrl = v.Link.Href
			} else {
				parsedUrl, err := url.Parse(v.Link.Href)

				if err == nil {
					videoUrl = strings.ReplaceAll(videoUrlTemplate, "{VIDEO-ID}", parsedUrl.Query().Get("v"))
				} else {
					videoUrl = "#"
				}
			}

			videos = append(videos, video{
				ThumbnailUrl: v.Group.Thumbnail.Url,
				Title:        v.Title,
				Url:          videoUrl,
				Author:       response.Channel,
				AuthorUrl:    response.ChannelLink + "/videos",
				TimePosted:   parseYoutubeFeedTime(v.Published),
			})
		}
	}

	if len(videos) == 0 {
		return nil, errNoContent
	}

	videos.sortByNewest()

	if failed > 0 {
		return videos, fmt.Errorf("%w: missing videos from %d channels", errPartialContent, failed)
	}

	return videos, nil
}
