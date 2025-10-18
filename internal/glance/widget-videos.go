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

const videosWidgetPlaylistPrefix = "playlist:"

var (
	videosWidgetTemplate             = mustParseTemplate("videos.html", "widget-base.html", "video-card-contents.html")
	videosWidgetGridTemplate         = mustParseTemplate("videos-grid.html", "widget-base.html", "video-card-contents.html")
	videosWidgetVerticalListTemplate = mustParseTemplate("videos-vertical-list.html", "widget-base.html")
)

type videosWidget struct {
	widgetBase                   `yaml:",inline"`
	Videos                       videoList `yaml:"-"`
	VideoUrlTemplate             string    `yaml:"video-url-template"`
	Style                        string    `yaml:"style"`
	CollapseAfter                int       `yaml:"collapse-after"`
	CollapseAfterRows            int       `yaml:"collapse-after-rows"`
	Channels                     []string  `yaml:"channels"`
	Playlists                    []string  `yaml:"playlists"`
	Limit                        int       `yaml:"limit"`
	IncludeShorts                bool      `yaml:"include-shorts"`
	UseDearrowTitles             bool      `yaml:"use-dearrow-titles"`
	UseDearrowThumbnails         bool      `yaml:"use-dearrow-thumbnails"`
	DearrowTitlesInstanceUrl     string    `yaml:"dearrow-titles-instance-url"`
	DearrowThumbnailsInstanceUrl string    `yaml:"dearrow-thumbnails-instance-url"`
}

func (widget *videosWidget) initialize() error {
	widget.withTitle("Videos").withCacheDuration(time.Hour)

	if widget.Limit <= 0 {
		widget.Limit = 25
	}

	if widget.CollapseAfterRows == 0 || widget.CollapseAfterRows < -1 {
		widget.CollapseAfterRows = 4
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 7
	}

	// A bit cheeky, but from a user's perspective it makes more sense when channels and
	// playlists are separate things rather than specifying a list of channels and some of
	// them awkwardly have a "playlist:" prefix
	if len(widget.Playlists) > 0 {
		initialLen := len(widget.Channels)
		widget.Channels = append(widget.Channels, make([]string, len(widget.Playlists))...)

		for i := range widget.Playlists {
			widget.Channels[initialLen+i] = videosWidgetPlaylistPrefix + widget.Playlists[i]
		}
	}

	return nil
}

func (widget *videosWidget) update(ctx context.Context) {
	videos, err := fetchYoutubeChannelUploads(widget.Channels, widget.VideoUrlTemplate, widget.IncludeShorts, widget.UseDearrowTitles, widget.UseDearrowThumbnails, widget.DearrowTitlesInstanceUrl, widget.DearrowThumbnailsInstanceUrl)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	if len(videos) > widget.Limit {
		videos = videos[:widget.Limit]
	}

	widget.Videos = videos
}

func (widget *videosWidget) Render() template.HTML {
	var template *template.Template

	switch widget.Style {
	case "grid-cards":
		template = videosWidgetGridTemplate
	case "vertical-list":
		template = videosWidgetVerticalListTemplate
	default:
		template = videosWidgetTemplate
	}

	return widget.renderTemplate(widget, template)
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

type dearrowBrandingResponseJson struct {
	Titles []struct {
		Title    string `json:"title"`
		Original bool   `json:"original"`
		Votes    int    `json:"votes"`
		Locked   bool   `json:"locked"`
		UUID     string `json:"UUID"`
		UserID   string `json:"userID,omitempty"`
	} `json:"titles"`
	Thumbnails []struct {
		Timestamp float32 `json:"timestamp"`
		Original  bool    `json:"original"`
		Votes     int     `json:"votes"`
		Locked    bool    `json:"locked"`
		UUID      string  `json:"UUID"`
		UserID    string  `json:"userID,omitempty"`
	} `json:"thumbnails"`
	RandomTime    float32  `json:"randomTime"`
	VideoDuration *float32 `json:"videoDuration"`
}

// https://wiki.sponsor.ajay.app/w/API_Docs/DeArrow
func dearrowGetFirstMatchingTitle(response dearrowBrandingResponseJson) (string, bool) {
	if len(response.Titles) > 0 {
		for _, title := range response.Titles {
			if title.Locked || title.Votes >= 0 {
				return title.Title, true
			}
		}
	}

	for _, thumbnail := range response.Thumbnails {
		if thumbnail.Locked || thumbnail.Votes >= 0 {
			return "", true
		}
	}

	return "", false
}

func dearrowGetThumbnailTask(client requestDoer) func(*http.Request) (string, error) {
	return func(request *http.Request) (string, error) {
		response, err := client.Do(request)
		if err != nil {
			return "", err
		}
		defer response.Body.Close()

		if response.StatusCode == http.StatusOK { // status code 200
			return request.URL.String(), nil
		}

		if response.StatusCode == http.StatusNoContent { // status code 204
			failReason := response.Header.Get("X-Failure-Reason")
			slog.Error("Failed to get the DeArrow thumbnail. Reason:", failReason, nil)
			return "", fmt.Errorf("no content, X-Failure-Reason: %s", failReason)
		}

		return "", nil
	}
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

func fetchYoutubeChannelUploads(channelOrPlaylistIDs []string, videoUrlTemplate string, includeShorts bool, useDearrowTitles bool, useDearrowThumbnails bool, dearrowTitlesInstanceUrl string, dearrowThumbnailsInstanceUrl string) (videoList, error) {
	requests := make([]*http.Request, 0, len(channelOrPlaylistIDs))

	for i := range channelOrPlaylistIDs {
		var feedUrl string
		if strings.HasPrefix(channelOrPlaylistIDs[i], videosWidgetPlaylistPrefix) {
			feedUrl = "https://www.youtube.com/feeds/videos.xml?playlist_id=" +
				strings.TrimPrefix(channelOrPlaylistIDs[i], videosWidgetPlaylistPrefix)
		} else if !includeShorts && strings.HasPrefix(channelOrPlaylistIDs[i], "UC") {
			playlistId := strings.Replace(channelOrPlaylistIDs[i], "UC", "UULF", 1)
			feedUrl = "https://www.youtube.com/feeds/videos.xml?playlist_id=" + playlistId
		} else {
			feedUrl = "https://www.youtube.com/feeds/videos.xml?channel_id=" + channelOrPlaylistIDs[i]
		}

		request, _ := http.NewRequest("GET", feedUrl, nil)
		requests = append(requests, request)
	}

	job := newJob(decodeXmlFromRequestTask[youtubeFeedResponseXml](defaultHTTPClient), requests).withWorkers(30)
	responses, errs, err := workerPoolDo(job)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errNoContent, err)
	}

	videos := make(videoList, 0, len(channelOrPlaylistIDs)*15)
	var failed int

	for i := range responses {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to fetch youtube feed", "channel", channelOrPlaylistIDs[i], "error", errs[i])
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

	if useDearrowTitles || useDearrowThumbnails {
		if dearrowTitlesInstanceUrl == "" {
			dearrowTitlesInstanceUrl = "https://sponsor.ajay.app"
		}

		if dearrowThumbnailsInstanceUrl == "" {
			dearrowThumbnailsInstanceUrl = "https://dearrow-thumb.ajay.app"
		}

		var dearrowTitleRequests []*http.Request
		var dearrowTitleIndices []int

		var dearrowThumbnailRequests []*http.Request
		var dearrowThumbnailIndices []int

		for i, vid := range videos {
			if vid.Url == "#" {
				continue
			}
			parsedUrl, err := url.Parse(vid.Url)
			if err != nil {
				slog.Error("Failed to parse video URL for dearrow", "url", vid.Url, "error", err)
				continue
			}
			videoID := parsedUrl.Query().Get("v")
			if videoID == "" {
				continue
			}

			if useDearrowTitles {
				req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/branding?videoID=%s", dearrowTitlesInstanceUrl, videoID), nil)
				if err != nil {
					slog.Error("Failed to create dearrow branding request", "videoID", videoID, "error", err)
					continue
				}
				dearrowTitleRequests = append(dearrowTitleRequests, req)
				dearrowTitleIndices = append(dearrowTitleIndices, i)
			}

			if useDearrowThumbnails {
				req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/getThumbnail?videoID=%s", dearrowThumbnailsInstanceUrl, videoID), nil)
				if err != nil {
					slog.Error("Failed to create dearrow branding request", "videoID", videoID, "error", err)
					continue
				}
				dearrowThumbnailRequests = append(dearrowThumbnailRequests, req)
				dearrowThumbnailIndices = append(dearrowThumbnailIndices, i)
			}
		}

		if useDearrowTitles {
			jobDearrowTitles := newJob(decodeJsonFromRequestTask[dearrowBrandingResponseJson](defaultHTTPClient), dearrowTitleRequests).withWorkers(30)
			dearrowTitleResponses, dearrowTitleErrs, err := workerPoolDo(jobDearrowTitles)
			if err != nil {
				slog.Error("Failed to complete dearrow branding job pool", "error", err)
			}

			for j, videoIndex := range dearrowTitleIndices {
				if dearrowTitleErrs[j] != nil {
					continue
				}

				brandingResponse := dearrowTitleResponses[j]
				if newTitle, ok := dearrowGetFirstMatchingTitle(brandingResponse); ok && newTitle != "" {
					videos[videoIndex].Title = newTitle
				}
			}
		}

		if useDearrowThumbnails {
			jobDearrowThumbnails := newJob(dearrowGetThumbnailTask(defaultHTTPClient), dearrowThumbnailRequests).withWorkers(30)
			dearrowThumbnailResponses, dearrowThumbnailErrs, err := workerPoolDo(jobDearrowThumbnails)
			if err != nil {
				slog.Error("Failed to complete dearrow getThumbnail job pool", "error", err)
			}

			for j, videoIndex := range dearrowThumbnailIndices {
				if dearrowThumbnailErrs[j] != nil {
					continue
				}

				thumbnailResponse := dearrowThumbnailResponses[j]
				if thumbnailResponse != "" {
					videos[videoIndex].ThumbnailUrl = thumbnailResponse
				}
			}
		}
	}

	videos.sortByNewest()

	if failed > 0 {
		return videos, fmt.Errorf("%w: missing videos from %d channels", errPartialContent, failed)
	}

	return videos, nil
}
