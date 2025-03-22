package glance

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var mediaServerTemplate = mustParseTemplate(
	"media-server.html",
	"widget-base.html",
)

type MediaServerWidget struct {
	widgetBase    `yaml:",inline"`
	MediaServer   string `yaml:"media-server"` // "plex", "jellyfin", or "tautulli"
	ApiKey        string `yaml:"apikey"`
	Url           string `yaml:"url"`
	DisplayPaused bool   `yaml:"display-paused"`
	ProgressBar   bool   `yaml:"progress-bar"`
	ProgressType  string `yaml:"progress-type"` // "ends-at-24", "ends-at-12" "percentage" or "none"
	Thumbnails    bool   `yaml:"thumbnails"`
	Users         Users  `yaml:"-"`
}

// constants
type MediaServerType string

const (
	MediaServerPlex     MediaServerType = "plex"
	MediaServerJellyfin MediaServerType = "jellyfin"
	MediaServerTautulli MediaServerType = "tautulli"
)

type ProgressType string

const (
	Default    ProgressType = ""
	EndsAt24   ProgressType = "ends-at-24"
	EndsAt12   ProgressType = "ends-at-12"
	Percentage ProgressType = "percentage"
	None       ProgressType = "none"
)

type MediaType string

const (
	MediaTypeMovie   MediaType = "movie"
	MediaTypeShow    MediaType = "show"
	MediaTypeMusic   MediaType = "music"
	MediaTypeUnknown MediaType = "unknown"
)

// interface for different media servers
type MediaServer interface {
	FetchSessions(ctx context.Context) (Users, error)
}

// media servers structs
type BaseMediaServer struct {
	baseURL    string
	apiKey     string
	timeFormat string
}

type TautulliServer struct {
	BaseMediaServer
}

type PlexServer struct {
	BaseMediaServer
}

type JellyfinServer struct {
	BaseMediaServer
}

// general structs
type Session struct {
	MediaType        string
	Title            string
	MovieTitle       string
	SeriesTitle      string
	SeasonNumber     string
	EpisodeTitle     string
	EpisodeNumber    string
	ArtistTitle      string
	AlbumTitle       string
	SongTitle        string
	IsPlaying        bool
	ProgressPercent  string
	RemainingSeconds string
	EndTime          string
	Thumb            string
}

type User struct {
	Name     string
	Sessions []Session
}

type Users []User

// initialize widget and validations
func (widget *MediaServerWidget) initialize() error {
	widget.
		withTitle("Media Server").
		withTitleURL(widget.Url).
		withCacheDuration(1 * time.Minute)

	// validate required fields
	if widget.ApiKey == "" {
		return fmt.Errorf("apikey is required")
	}
	if widget.Url == "" {
		return fmt.Errorf("url is required")
	}

	// validate media server type
	switch MediaServerType(widget.MediaServer) {
	case MediaServerPlex, MediaServerJellyfin, MediaServerTautulli:
	default:
		return fmt.Errorf(
			"invalid media server type: %s. Must be one of: plex, jellyfin, tautulli",
			widget.MediaServer,
		)
	}

	// validate progress type
	switch ProgressType(widget.ProgressType) {
	case Default:
		widget.ProgressType = string(EndsAt24)
	case EndsAt24, EndsAt12, Percentage, None:
	default:
		return fmt.Errorf(
			"invalid progress type: %s. Must be one of: ends-at-24, ends-at-12, percentage, none",
			widget.ProgressType,
		)
	}

	return nil
}

// fetch and process data from media server
func (widget *MediaServerWidget) update(ctx context.Context) {
	var users Users
	var err error
	timeFormat := getTimeFormat(widget.ProgressType)

	switch MediaServerType(widget.MediaServer) {
	case MediaServerTautulli:
		server := &TautulliServer{
			BaseMediaServer: BaseMediaServer{
				baseURL:    widget.Url,
				apiKey:     widget.ApiKey,
				timeFormat: timeFormat,
			},
		}
		users, err = server.FetchSessions(ctx)
	case MediaServerJellyfin:
		server := &JellyfinServer{
			BaseMediaServer: BaseMediaServer{
				baseURL:    widget.Url,
				apiKey:     widget.ApiKey,
				timeFormat: timeFormat,
			},
		}
		users, err = server.FetchSessions(ctx)
	case MediaServerPlex:
		server := &PlexServer{
			BaseMediaServer: BaseMediaServer{
				baseURL:    widget.Url,
				apiKey:     widget.ApiKey,
				timeFormat: timeFormat,
			},
		}
		users, err = server.FetchSessions(ctx)
	default:
		// This should never happen because of the validation in initialize()
		err = fmt.Errorf("unsupported media server type: %s", widget.MediaServer)
	}

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.Users = users
}

// render the widget template
func (widget *MediaServerWidget) Render() template.HTML {
	return widget.renderTemplate(widget, mediaServerTemplate)
}

// Helper functions

func NewBaseMediaServer(baseURL, apiKey string) BaseMediaServer {
	return BaseMediaServer{
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

func fetchMediaServerData[T any](serverType, baseURL, apiKey string) (T, error) {
	var empty T

	var url string
	switch serverType {
	case string(MediaServerTautulli):
		url = fmt.Sprintf(
			"%s/api/v2?apikey=%s&cmd=get_activity",
			baseURL, apiKey,
		)
	case string(MediaServerPlex):
		url = fmt.Sprintf(
			"%s/status/sessions?X-Plex-Token=%s",
			baseURL, apiKey,
		)
	case string(MediaServerJellyfin):
		url = fmt.Sprintf(
			"%s/Sessions?activeWithinSeconds=900&api_key=%s",
			baseURL, apiKey,
		)
	default:
		return empty, fmt.Errorf("unknown server type: %s", serverType)
	}

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return empty, fmt.Errorf("could not create request for %s: %w", serverType, err)
	}

	// add headers if needed
	if serverType == string(MediaServerPlex) {
		request.Header.Add("Accept", "application/json")
	}

	response, err := decodeJsonFromRequest[T](defaultHTTPClient, request)
	if err != nil {
		return empty, fmt.Errorf("could not fetch %s's sessions: %w", serverType, errNoContent)
	}

	return response, nil
}

// convert server-specific media type strings to general MediaType
func determineMediaType(mediaType string) MediaType {
	switch strings.ToLower(mediaType) {
	case "movie":
		return MediaTypeMovie
	case "episode":
		return MediaTypeShow
	case "track", "audio":
		return MediaTypeMusic
	default:
		return MediaTypeUnknown
	}
}

// get the playing state
func getPlayingState(state string) bool {
	var isPlaying bool

	switch strings.ToLower(state) {
	case "playing", "true":
		isPlaying = true
	default:
		isPlaying = false
	}

	return isPlaying
}

// get the prefered time format
func getTimeFormat(progressType string) string {
	if progressType == string(EndsAt12) {
		return "03:04 PM"
	}
	return "15:04"
}

// calculate progress percentage, remaining time and ending time
func calculateProgress(current, total float64, timeFormat string) (string, string, string) {
	currentTime := time.Now()

	if total <= 0 {
		return "0", "0", currentTime.Format(timeFormat)
	}

	progressPercent := fmt.Sprintf("%.0f", (current*100)/total)

	remainingSeconds := int((total - current) / 1000)
	remainingSecondsStr := fmt.Sprintf("%d", remainingSeconds)

	endTime := currentTime.Add(time.Duration(remainingSeconds) * time.Second)
	endTimeStr := endTime.Format(timeFormat)

	return progressPercent, remainingSecondsStr, endTimeStr
}

// generate a thumbnail URL based on the server type
func (b *BaseMediaServer) generateThumbnailURL(
	serverType MediaServerType,
	mediaType MediaType,
	itemID, parentID string,
) string {
	switch serverType {
	case MediaServerTautulli:
		imgPath := itemID
		if mediaType == MediaTypeShow {
			imgPath = parentID
		}
		return fmt.Sprintf(
			"%s/api/v2?apikey=%s&cmd=pms_image_proxy&width=50&img=%s",
			b.baseURL, b.apiKey, imgPath,
		)
	case MediaServerPlex:
		imgPath := itemID
		if mediaType == MediaTypeShow {
			imgPath = parentID
		}
		return fmt.Sprintf(
			"%s%s?X-Plex-Token=%s",
			b.baseURL, imgPath, b.apiKey,
		)
	case MediaServerJellyfin:
		imgPath := itemID
		if mediaType == MediaTypeShow {
			imgPath = parentID
		}
		return fmt.Sprintf(
			"%s/Items/%s/Images/Primary?maxWidth=50&api_key=%s",
			b.baseURL, imgPath, b.apiKey,
		)
	default:
		return ""
	}
}

// add a session to a user in the userMap
func addSessionToUser(userMap map[string]*User, username string, session Session) {
	if user, exists := userMap[username]; exists {
		user.Sessions = append(user.Sessions, session)
	} else {
		userMap[username] = &User{
			Name:     username,
			Sessions: []Session{session},
		}
	}
}

// convert a map of users to a slice
func userMapToSlice(userMap map[string]*User) Users {
	users := make(Users, 0, len(userMap))
	for _, user := range userMap {
		users = append(users, *user)
	}
	return users
}

// tautulli

type tautulliGetActivityJson struct {
	Response struct {
		Data struct {
			StreamCount string `json:"stream_count"`
			Sessions    []struct {
				User             string `json:"user"`
				MediaType        string `json:"media_type"`
				GrandparentTitle string `json:"grandparent_title"`
				ParentTitle      string `json:"parent_title"`
				Title            string `json:"title"`
				ParentMediaIndex string `json:"parent_media_index"`
				MediaIndex       string `json:"media_index"`
				State            string `json:"state"`
				ViewOffset       string `json:"view_offset"`
				Duration         string `json:"duration"`
				Thumb            string `json:"thumb"`
				GrandparentThumb string `json:"grandparent_thumb"`
			} `json:"sessions"`
		} `json:"data"`
	} `json:"response"`
}

func (t *TautulliServer) FetchSessions(ctx context.Context) (Users, error) {
	response, err := fetchMediaServerData[tautulliGetActivityJson](
		string(MediaServerTautulli),
		t.baseURL,
		t.apiKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Tautulli sessions: %w", err)
	}

	userMap := make(map[string]*User)

	for _, session := range response.Response.Data.Sessions {
		isPlaying := getPlayingState(session.State)

		viewOffset, _ := strconv.ParseFloat(session.ViewOffset, 64)
		duration, _ := strconv.ParseFloat(session.Duration, 64)
		progressPercent, remainingSeconds, endTime := calculateProgress(
			viewOffset,
			duration,
			t.timeFormat,
		)
		mediaType := determineMediaType(session.MediaType)
		thumbUrl := t.generateThumbnailURL(
			MediaServerTautulli,
			mediaType,
			session.Thumb,
			session.GrandparentThumb,
		)

		sessionData := Session{
			MediaType:        string(mediaType),
			Title:            session.Title,
			MovieTitle:       session.Title,
			SeriesTitle:      session.GrandparentTitle,
			SeasonNumber:     session.ParentMediaIndex,
			EpisodeTitle:     session.Title,
			EpisodeNumber:    session.MediaIndex,
			ArtistTitle:      session.GrandparentTitle,
			AlbumTitle:       session.ParentTitle,
			SongTitle:        session.Title,
			IsPlaying:        isPlaying,
			ProgressPercent:  progressPercent,
			RemainingSeconds: remainingSeconds,
			EndTime:          endTime,
			Thumb:            thumbUrl,
		}

		addSessionToUser(userMap, session.User, sessionData)
	}

	return userMapToSlice(userMap), nil
}

// plex

type plexGetActivityJson struct {
	MediaContainer struct {
		Metadata []struct {
			User struct {
				Title string `json:"title"`
			} `json:"User"`
			Player struct {
				State string `json:"state"`
			} `json:"Player"`
			Type             string `json:"type"`
			Title            string `json:"title"`
			ParentTitle      string `json:"parentTitle"`
			GrandparentTitle string `json:"grandparentTitle"`
			Index            int    `json:"index"`
			ParentIndex      int    `json:"parentIndex"`
			Thumb            string `json:"thumb"`
			GrandparentThumb string `json:"grandparentThumb"`
			ViewOffset       int    `json:"viewOffset"`
			Duration         int    `json:"duration"`
		} `json:"Metadata"`
	} `json:"MediaContainer"`
}

func (p *PlexServer) FetchSessions(ctx context.Context) (Users, error) {
	response, err := fetchMediaServerData[plexGetActivityJson](
		string(MediaServerPlex),
		p.baseURL,
		p.apiKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Plex sessions: %w", err)
	}

	userMap := make(map[string]*User)

	for _, session := range response.MediaContainer.Metadata {
		isPlaying := getPlayingState(session.Player.State)

		progressPercent, remainingSeconds, endTime := calculateProgress(
			float64(session.ViewOffset),
			float64(session.Duration),
			p.timeFormat,
		)
		mediaType := determineMediaType(session.Type)
		thumbUrl := p.generateThumbnailURL(
			MediaServerPlex,
			mediaType,
			session.Thumb,
			session.GrandparentThumb,
		)

		sessionData := Session{
			MediaType:        string(mediaType),
			Title:            session.Title,
			MovieTitle:       session.Title,
			SeriesTitle:      session.GrandparentTitle,
			SeasonNumber:     fmt.Sprintf("%d", session.ParentIndex),
			EpisodeTitle:     session.Title,
			EpisodeNumber:    fmt.Sprintf("%d", session.Index),
			ArtistTitle:      session.GrandparentTitle,
			AlbumTitle:       session.ParentTitle,
			SongTitle:        session.Title,
			IsPlaying:        isPlaying,
			ProgressPercent:  progressPercent,
			RemainingSeconds: remainingSeconds,
			EndTime:          endTime,
			Thumb:            thumbUrl,
		}

		addSessionToUser(userMap, session.User.Title, sessionData)
	}

	return userMapToSlice(userMap), nil
}

// jellyfin

type jellyfinGetActivityJson []struct {
	UserName  string `json:"UserName"`
	PlayState struct {
		IsPaused      bool `json:"IsPaused"`
		PositionTicks int  `json:"PositionTicks"`
	} `json:"PlayState"`
	NowPlayingItem struct {
		RunTimeTicks      int    `json:"RunTimeTicks"`
		Type              string `json:"Type"`
		Name              string `json:"Name"`
		Album             string `json:"Album"`
		AlbumArtist       string `json:"AlbumArtist"`
		SeriesName        string `json:"SeriesName"`
		IndexNumber       int    `json:"IndexNumber"`
		ParentIndexNumber int    `json:"ParentIndexNumber"`
		Id                string `json:"Id"`
		SeriesId          string `json:"SeriesId"`
	} `json:"NowPlayingItem"`
}

func (j *JellyfinServer) FetchSessions(ctx context.Context) (Users, error) {
	response, err := fetchMediaServerData[jellyfinGetActivityJson](
		string(MediaServerJellyfin),
		j.baseURL,
		j.apiKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Jellyfin sessions: %w", err)
	}

	userMap := make(map[string]*User)

	for _, session := range response {
		progressPercent, remainingSeconds, endTime := calculateProgress(
			float64(session.PlayState.PositionTicks),
			float64(session.NowPlayingItem.RunTimeTicks),
			j.timeFormat,
		)
		mediaType := determineMediaType(session.NowPlayingItem.Type)
		thumbUrl := j.generateThumbnailURL(
			MediaServerJellyfin,
			mediaType,
			session.NowPlayingItem.Id,
			session.NowPlayingItem.SeriesId,
		)

		sessionData := Session{
			MediaType:        string(mediaType),
			Title:            session.NowPlayingItem.Name,
			MovieTitle:       session.NowPlayingItem.Name,
			SeriesTitle:      session.NowPlayingItem.SeriesName,
			SeasonNumber:     fmt.Sprintf("%d", session.NowPlayingItem.ParentIndexNumber),
			EpisodeTitle:     session.NowPlayingItem.Name,
			EpisodeNumber:    fmt.Sprintf("%d", session.NowPlayingItem.IndexNumber),
			ArtistTitle:      session.NowPlayingItem.AlbumArtist,
			AlbumTitle:       session.NowPlayingItem.Album,
			SongTitle:        session.NowPlayingItem.Name,
			IsPlaying:        !session.PlayState.IsPaused,
			ProgressPercent:  progressPercent,
			RemainingSeconds: remainingSeconds,
			EndTime:          endTime,
			Thumb:            thumbUrl,
		}

		addSessionToUser(userMap, session.UserName, sessionData)
	}

	return userMapToSlice(userMap), nil
}
