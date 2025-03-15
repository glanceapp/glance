package glance

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"
)

var lastFmTemplate = mustParseTemplate("last-fm.html", "widget-base.html")

type lastFmWidget struct {
	widgetBase    `yaml:",inline"`
	Tracks        lastFmTrackList `yaml:"-"`
	ApiKey        string          `yaml:"api-key"`
	Username      string          `yaml:"username"`
	Limit         int             `yaml:"limit"`
	CollapseAfter int             `yaml:"collapse-after"`
	SquareCover   bool            `yaml:"square-cover"`
	StaticCover   bool            `yaml:"static-cover"`
}

func (widget *lastFmWidget) initialize() error {
	widget.
		withTitle("Last.FM").
		withTitleURL(fmt.Sprintf("https://www.last.fm/user/%s", widget.Username)).
		withCacheDuration(1 * time.Minute)

	if widget.Limit <= 0 {
		widget.Limit = 1
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 1
	}

	return nil
}

func (widget *lastFmWidget) update(ctx context.Context) {
	tracks, err := fetchLastFmRecentTracks(widget.ApiKey, widget.Username, widget.Limit)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.Tracks = tracks
}

func (widget *lastFmWidget) Render() template.HTML {
	return widget.renderTemplate(widget, lastFmTemplate)
}

type lastFmResponseJson struct {
	RecentTracks struct {
		TrackList []struct {
			Artist struct {
				Text string `json:"#text"`
			} `json:"artist"`
			Name  string `json:"name"`
			Image []struct {
				Text string `json:"#text"`
			} `json:"image"`
			Url        string `json:"url"`
			Attributes *struct {
				NowPlaying string `json:"nowplaying"`
			} `json:"@attr"`
			Date *struct {
				Timestamp string `json:"uts"`
			} `json:"date"`
		} `json:"track"`
	} `json:"recenttracks"`
}

type lastFmTrack struct {
	Song      string
	Artist    string
	Image     string
	Url       string
	IsPlaying bool
	PlayedAt  time.Time
}

type lastFmTrackList []lastFmTrack

func fetchLastFmRecentTracks(apiKey string, username string, limit int) (lastFmTrackList, error) {
	// The limit is increased by one to account for the currently playing song
	url := fmt.Sprintf("http://ws.audioscrobbler.com/2.0/?method=user.getrecenttracks&user=%s&api_key=%s&format=json&limit=%d", username, apiKey, limit+1)

	request, _ := http.NewRequest("GET", url, nil)
	response, err := decodeJsonFromRequest[lastFmResponseJson](defaultHTTPClient, request)
	if err != nil {
		return nil, fmt.Errorf("%w: could not fetch recent last.fm tracks", errNoContent)
	}

	tracks := make(lastFmTrackList, 0, limit)

	for _, lastFmData := range response.RecentTracks.TrackList {
		var playedAt time.Time
		if lastFmData.Date != nil {
			// Convert string unix time to number
			timestamp, err := strconv.Atoi(lastFmData.Date.Timestamp)
			if err != nil {
				return nil, fmt.Errorf("%w: could not convert timestamp to number", errNoContent)
			}

			playedAt = time.Unix(int64(timestamp), 0)
		}

		tracks = append(tracks, lastFmTrack{
			Song:      lastFmData.Name,
			Artist:    lastFmData.Artist.Text,
			Image:     lastFmData.Image[1].Text,
			Url:       lastFmData.Url,
			IsPlaying: lastFmData.Attributes != nil,
			PlayedAt:  playedAt,
		})
	}

	// If no song is playing, remove the last track from the list
	// See above for context (url variable)
	if len(tracks) > limit && !tracks[len(tracks)-1].IsPlaying {
		tracks = tracks[:limit]
	}

	if len(tracks) == 0 {
		return nil, errNoContent
	}

	return tracks, nil
}
