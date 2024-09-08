package feed

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

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

func FetchYoutubeChannelUploads(channelIds []string, videoUrlTemplate string, includeShorts bool) (Videos, error) {
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
		return nil, fmt.Errorf("%w: %v", ErrNoContent, err)
	}

	videos := make(Videos, 0, len(channelIds)*15)

	var failed int

	for i := range responses {
		if errs[i] != nil {
			failed++
			slog.Error("Failed to fetch youtube feed", "channel", channelIds[i], "error", errs[i])
			continue
		}

		response := responses[i]

		for j := range response.Videos {
			video := &response.Videos[j]
			var videoUrl string

			if videoUrlTemplate == "" {
				videoUrl = video.Link.Href
			} else {
				parsedUrl, err := url.Parse(video.Link.Href)

				if err == nil {
					videoUrl = strings.ReplaceAll(videoUrlTemplate, "{VIDEO-ID}", parsedUrl.Query().Get("v"))
				} else {
					videoUrl = "#"
				}
			}

			videos = append(videos, Video{
				ThumbnailUrl: video.Group.Thumbnail.Url,
				Title:        video.Title,
				Url:          videoUrl,
				Author:       response.Channel,
				AuthorUrl:    response.ChannelLink + "/videos",
				TimePosted:   parseYoutubeFeedTime(video.Published),
			})
		}
	}

	if len(videos) == 0 {
		return nil, ErrNoContent
	}

	videos.SortByNewest()

	if failed > 0 {
		return videos, fmt.Errorf("%w: missing videos from %d channels", ErrPartialContent, failed)
	}

	return videos, nil
}
