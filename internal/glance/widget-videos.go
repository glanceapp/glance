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
	"sync"
	"time"
)

const videosWidgetPlaylistPrefix = "playlist:"

var (
	videosWidgetTemplate             = mustParseTemplate("videos.html", "widget-base.html", "video-card-contents.html")
	videosWidgetGridTemplate         = mustParseTemplate("videos-grid.html", "widget-base.html", "video-card-contents.html")
	videosWidgetVerticalListTemplate = mustParseTemplate("videos-vertical-list.html", "widget-base.html")
)

type videosWidget struct {
	widgetBase        `yaml:",inline"`
	Videos            videoList `yaml:"-"`
	cachedVideoLists  sync.Map  `yaml:"-"`
	VideoUrlTemplate  string    `yaml:"video-url-template"`
	Style             string    `yaml:"style"`
	CollapseAfter     int       `yaml:"collapse-after"`
	CollapseAfterRows int       `yaml:"collapse-after-rows"`
	Channels          []string  `yaml:"channels"`
	Playlists         []string  `yaml:"playlists"`
	Limit             int       `yaml:"limit"`
	IncludeShorts     bool      `yaml:"include-shorts"`
	SortBy            string    `yaml:"sort-by"`
	YoutubeAPIKey     string    `yaml:"youtube-api-key"`

	Filters filterableFields[video] `yaml:"filters"`
}

func (w *videosWidget) initialize() error {
	w.withTitle("Videos").withCacheDuration(time.Hour)

	if w.Limit <= 0 {
		w.Limit = 25
	}

	if w.CollapseAfterRows == 0 || w.CollapseAfterRows < -1 {
		w.CollapseAfterRows = 4
	}

	if w.CollapseAfter == 0 || w.CollapseAfter < -1 {
		w.CollapseAfter = 7
	}

	// A bit cheeky, but from a user's perspective it makes more sense when channels and
	// playlists are separate things rather than specifying a list of channels and some of
	// them awkwardly have a "playlist:" prefix
	if len(w.Playlists) > 0 {
		initialLen := len(w.Channels)
		w.Channels = append(w.Channels, make([]string, len(w.Playlists))...)

		for i := range w.Playlists {
			w.Channels[initialLen+i] = videosWidgetPlaylistPrefix + w.Playlists[i]
		}
	}

	return nil
}

func (w *videosWidget) update(ctx context.Context) {
	videos, err := w.fetchYoutubeChannelUploads(w.Channels, w.VideoUrlTemplate, w.IncludeShorts, w.SortBy)

	if !w.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	videos = w.Filters.Apply(videos)

	if len(videos) > w.Limit {
		videos = videos[:w.Limit]
	}

	w.Videos = videos
}

func (w *videosWidget) Render() template.HTML {
	var template *template.Template

	switch w.Style {
	case "grid-cards":
		template = videosWidgetGridTemplate
	case "vertical-list":
		template = videosWidgetVerticalListTemplate
	default:
		template = videosWidgetTemplate
	}

	return w.renderTemplate(w, template)
}

type youtubeFeedResponseXml struct {
	Channel     string `xml:"author>name"`
	ChannelLink string `xml:"author>uri"`
	Videos      []struct {
		Title     string `xml:"title"`
		Published string `xml:"published"`
		Updated   string `xml:"updated"`
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
	TimeUpdated  time.Time
}

func (v video) filterableField(field string) any {
	switch field {
	case "title":
		return v.Title
	case "posted":
		return v.TimePosted
	case "updated":
		return v.TimeUpdated
	default:
		return nil
	}
}

type videoList []video

func (v videoList) sortByPosted() videoList {
	sort.Slice(v, func(i, j int) bool {
		return v[i].TimePosted.After(v[j].TimePosted)
	})

	return v
}

func (v videoList) sortByUpdated() videoList {
	sort.Slice(v, func(i, j int) bool {
		return v[i].TimeUpdated.After(v[j].TimeUpdated)
	})

	return v
}

func (w *videosWidget) fetchYoutubeChannelUploads(channelOrPlaylistIDs []string, videoUrlTemplate string, includeShorts bool, sortBy string) (videoList, error) {
	task := func(id string) (videoList, error) {
		if cached, ok := w.cachedVideoLists.Load(id); ok {
			entry := cached.(cachedEntry[videoList])
			if time.Since(entry.timestamp) < w.cacheDuration {
				return entry.value, nil
			}
		}

		var list videoList
		var err error
		if w.YoutubeAPIKey != "" {
			list, err = fetchYoutubePlaylistItems(youtubeWidgetPlaylistID(id, includeShorts), w.YoutubeAPIKey, videoUrlTemplate)
		} else {
			list, err = fetchYoutubeFeed(id, includeShorts, videoUrlTemplate)
		}

		if err != nil {
			cached, ok := w.cachedVideoLists.Load(id)
			if ok {
				return cached.(cachedEntry[videoList]).value, err
			}

			return list, err
		}

		w.cachedVideoLists.Store(id, cachedEntry[videoList]{value: list, timestamp: time.Now()})
		return list, nil
	}

	job := newJob(task, channelOrPlaylistIDs).withWorkers(30)
	lists, errs, err := workerPoolDo(job)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errNoContent, err)
	}

	videos := make(videoList, 0, len(channelOrPlaylistIDs)*15)
	var failed int

	for i := range lists {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to fetch youtube feed", "channel", channelOrPlaylistIDs[i], "error", errs[i])
		}

		// We still append the list even if it failed, because we may have a cached version of the list
		videos = append(videos, lists[i]...)
	}

	if len(videos) == 0 {
		return nil, errNoContent
	}

	switch sortBy {
	case "none":
	case "updated":
		videos.sortByUpdated()
	case "posted":
		videos.sortByPosted()
	default: // "posted"
		videos.sortByPosted()
	}

	if failed > 0 {
		return videos, fmt.Errorf("%w: missing videos from %d channels", errPartialContent, failed)
	}

	return videos, nil
}

func fetchYoutubeFeed(id string, includeShorts bool, videoUrlTemplate string) (videoList, error) {
	list := make(videoList, 0, 15)

	var feedURL string
	if after, ok := strings.CutPrefix(id, videosWidgetPlaylistPrefix); ok {
		feedURL = "https://www.youtube.com/feeds/videos.xml?playlist_id=" + after
	} else if !includeShorts && strings.HasPrefix(id, "UC") {
		playlistId := strings.Replace(id, "UC", "UULF", 1)
		feedURL = "https://www.youtube.com/feeds/videos.xml?playlist_id=" + playlistId
	} else {
		feedURL = "https://www.youtube.com/feeds/videos.xml?channel_id=" + id
	}

	request, _ := http.NewRequest("GET", feedURL, nil)
	response, err := decodeXmlFromRequest[youtubeFeedResponseXml](defaultHTTPClient, request)
	if err != nil {
		return list, err
	}

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

		list = append(list, video{
			ThumbnailUrl: v.Group.Thumbnail.Url,
			Title:        v.Title,
			Url:          videoUrl,
			Author:       response.Channel,
			AuthorUrl:    response.ChannelLink + "/videos",
			TimePosted:   parseYoutubeFeedTime(v.Published),
			TimeUpdated:  parseYoutubeFeedTime(v.Updated),
		})
	}

	return list, nil
}

// youtubeWidgetPlaylistID maps a widget entry to the playlist the Data API reads:
// the playlist itself, or the channel's uploads (UU) / long-form uploads (UULF) playlist.
func youtubeWidgetPlaylistID(id string, includeShorts bool) string {
	if after, ok := strings.CutPrefix(id, videosWidgetPlaylistPrefix); ok {
		return after
	}

	if !includeShorts {
		return strings.Replace(id, "UC", "UULF", 1)
	}

	return strings.Replace(id, "UC", "UU", 1)
}

type youtubePlaylistItemsResponseJson struct {
	Items []struct {
		Snippet struct {
			Title                  string `json:"title"`
			VideoOwnerChannelTitle string `json:"videoOwnerChannelTitle"`
			VideoOwnerChannelId    string `json:"videoOwnerChannelId"`
			Thumbnails             map[string]struct {
				Url string `json:"url"`
			} `json:"thumbnails"`
		} `json:"snippet"`
		ContentDetails struct {
			VideoId          string `json:"videoId"`
			VideoPublishedAt string `json:"videoPublishedAt"`
		} `json:"contentDetails"`
	} `json:"items"`
}

// fetchYoutubePlaylistItems reads a playlist through the YouTube Data API (1 quota unit
// per call), for when the public RSS feeds are unavailable.
func fetchYoutubePlaylistItems(playlistId string, apiKey string, videoUrlTemplate string) (videoList, error) {
	query := url.Values{}
	query.Set("part", "snippet,contentDetails")
	query.Set("maxResults", "15")
	query.Set("playlistId", playlistId)

	request, _ := http.NewRequest("GET", "https://www.googleapis.com/youtube/v3/playlistItems?"+query.Encode(), nil)
	// Sent as a header rather than a query parameter so the key never ends up in error logs
	request.Header.Set("X-Goog-Api-Key", apiKey)

	response, err := decodeJsonFromRequest[youtubePlaylistItemsResponseJson](defaultHTTPClient, request)
	if err != nil {
		return nil, err
	}

	list := make(videoList, 0, len(response.Items))
	for i := range response.Items {
		item := &response.Items[i]

		// Private and deleted videos stay in playlists but have no publish date
		if item.ContentDetails.VideoPublishedAt == "" {
			continue
		}

		videoId := item.ContentDetails.VideoId
		videoUrl := "https://www.youtube.com/watch?v=" + videoId
		if videoUrlTemplate != "" {
			videoUrl = strings.ReplaceAll(videoUrlTemplate, "{VIDEO-ID}", videoId)
		}

		var thumbnailUrl string
		for _, size := range []string{"high", "medium", "default"} {
			if thumbnail, ok := item.Snippet.Thumbnails[size]; ok {
				thumbnailUrl = thumbnail.Url
				break
			}
		}

		posted, err := time.Parse(time.RFC3339, item.ContentDetails.VideoPublishedAt)
		if err != nil {
			posted = time.Now()
		}

		list = append(list, video{
			ThumbnailUrl: thumbnailUrl,
			Title:        item.Snippet.Title,
			Url:          videoUrl,
			Author:       item.Snippet.VideoOwnerChannelTitle,
			AuthorUrl:    "https://www.youtube.com/channel/" + item.Snippet.VideoOwnerChannelId + "/videos",
			TimePosted:   posted,
			TimeUpdated:  posted,
		})
	}

	return list, nil
}
