package glance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var navidromeWidgetTemplate = mustParseTemplate("navidrome.html", "widget-base.html")

type navidromeWidget struct {
	widgetBase `yaml:",inline"`

	URL           string `yaml:"url"`
	Username      string `yaml:"user"`
	Password      string `yaml:"pass"`
	AllowInsecure bool `yaml:"allow-insecure"`
	ShowArtist    *bool `yaml:"artist"`
	ShowAlbum     *bool `yaml:"album"`
	ShowTrack     *bool `yaml:"track"`

	NowPlaying *navidromeNowPlaying `yaml:"-"`
}

type navidromeNowPlaying struct {
	IsPlaying       bool
	MinutesAgo      int
	Title           string
	Artist          string
	Album           string
	Track           int
	PositionSeconds int
	ProgressPercent int
	CoverArtURL     string
}

type navidromeResponse struct {
	SubsonicResponse struct {
		NowPlaying struct {
			Entry navidromeEntryList `json:"entry"`
		} `json:"nowPlaying"`
	} `json:"subsonic-response"`
}

type navidromeEntryList []navidromeEntry

func (entries *navidromeEntryList) UnmarshalJSON(data []byte) error {
	var list []navidromeEntry
	if err := json.Unmarshal(data, &list); err == nil {
		*entries = list
		return nil
	}

	var single navidromeEntry
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}

	*entries = []navidromeEntry{single}
	return nil
}

type navidromeEntry struct {
	State      string `json:"state"`
	MinutesAgo int    `json:"minutesAgo"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	Track      int    `json:"track"`
	PositionMs int    `json:"positionMs"`
	Duration   int    `json:"duration"`
	CoverArt   string `json:"coverArt"`
}

func boolPtrDefault(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func (widget *navidromeWidget) initialize() error {
	widget.
		withTitle("Navidrome").
		withTitleURL(strings.TrimRight(widget.URL, "/")).
		withCacheDuration(15 * time.Second)

	return nil
}

func (widget *navidromeWidget) update(ctx context.Context) {
	nowPlaying, err := fetchNavidromeNowPlaying(
		widget.URL,
		widget.AllowInsecure,
		widget.Username,
		widget.Password,
		)

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.NowPlaying = nowPlaying

	if widget.CustomCacheDuration == 0 {
		switch {
		case nowPlaying != nil && nowPlaying.IsPlaying:
			widget.cacheDuration = 10 * time.Second
		case nowPlaying != nil:
			widget.cacheDuration = time.Minute
		default:
			widget.cacheDuration = 5 * time.Minute
		}
	}
}

func (widget *navidromeWidget) Render() template.HTML {
	return widget.renderTemplate(widget, navidromeWidgetTemplate)
}

func (widget *navidromeWidget) MetadataLine() string {
	if widget.NowPlaying == nil {
		return ""
	}

	return formatNavidromeMetadata(
		widget.NowPlaying.Artist,
		widget.NowPlaying.Album,
		widget.NowPlaying.Track,
		boolPtrDefault(widget.ShowArtist, true),
		boolPtrDefault(widget.ShowAlbum, true),
		boolPtrDefault(widget.ShowTrack, false),
		)
}

func formatNavidromeMetadata(artist, album string, track int, showArtist, showAlbum, showTrack bool) string {
	var parts []string

	if showArtist && artist != "" {
		parts = append(parts, artist)
	}

	if showAlbum && album != "" {
		parts = append(parts, album)
	}

	line := strings.Join(parts, " - ")

	if showTrack && track > 0 {
		if line != "" {
			line += fmt.Sprintf(" (%d)", track)
		} else {
			line = fmt.Sprintf("(%d)", track)
		}
	}

	return line
}

func fetchNavidromeNowPlaying(instanceURL string, allowInsecure bool, username, password string) (*navidromeNowPlaying, error) {
	if username == "" || password == "" {
		return nil, errors.New("missing credentials")
	}

	requestURL := strings.TrimRight(instanceURL, "/") +
	"/rest/getNowPlaying?u=" + url.QueryEscape(username) +
	"&p=" + url.QueryEscape(password) +
	"&v=1.16.1&c=glance&f=json"

	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}

	client := ternary(allowInsecure, defaultInsecureHTTPClient, defaultHTTPClient)
	response, err := decodeJsonFromRequest[navidromeResponse](client, request)
	if err != nil {
		return nil, err
	}

	entries := response.SubsonicResponse.NowPlaying.Entry
	if len(entries) == 0 {
		return nil, nil
	}

	entry := entries[0]
	nowPlaying := &navidromeNowPlaying{
		MinutesAgo: entry.MinutesAgo,
		Title:      entry.Title,
		Artist:     entry.Artist,
		Album:      entry.Album,
		Track:      entry.Track,
		IsPlaying:  entry.State == "playing",
	}

	if entry.PositionMs > 0 {
		nowPlaying.PositionSeconds = entry.PositionMs / 1000
	}

	durationMs := entry.Duration * 1000
	if durationMs > 0 && entry.PositionMs > 0 {
		nowPlaying.ProgressPercent = entry.PositionMs * 100 / durationMs
	}

	if entry.CoverArt != "" {
		nowPlaying.CoverArtURL = buildNavidromeCoverArtURL(instanceURL, entry.CoverArt, username, password)
	}

	return nowPlaying, nil
}

func buildNavidromeCoverArtURL(instanceURL, coverArtID, username, password string) string {
	return strings.TrimRight(instanceURL, "/") +
	"/rest/getCoverArt?id=" + url.QueryEscape(coverArtID) +
	"&u=" + url.QueryEscape(username) +
	"&p=" + url.QueryEscape(password) +
	"&v=1.16.1&c=glance&f=json"
}

