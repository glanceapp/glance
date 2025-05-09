package glance

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	// "log"
	"net/http"
	"strings"
	"time"
)

var mediaRequestsWidgetTemplate = mustParseTemplate("media-requests.html", "widget-base.html")

type mediaRequestsWidget struct {
	widgetBase `yaml:",inline"`

	MediaRequests []MediaRequest `yaml:"-"`

	Service       string `yaml:"service"`
	URL           string `yaml:"url"`
	ApiKey        string `yaml:"api-key"`
	Limit         int    `yaml:"limit"`
	CollapseAfter int    `yaml:"collapse-after"`
}

func (widget *mediaRequestsWidget) initialize() error {
	widget.
		withTitle("Media Requests").
		withTitleURL(string(widget.URL)).
		withCacheDuration(10 * time.Minute)

	if widget.Service != "jellyseerr" && widget.Service != "overseerr" {
		return errors.New("service must be either 'jellyseerr' or 'overseerr'")
	}

	if widget.Limit <= 0 {
		widget.Limit = 20
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	return nil
}

func (widget *mediaRequestsWidget) update(ctx context.Context) {
	mediaReqs, err := fetchMediaRequests(widget.URL, widget.ApiKey, widget.Limit)
	if err != nil {
		return
	}

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.MediaRequests = mediaReqs

}

func (widget *mediaRequestsWidget) Render() template.HTML {
	return widget.renderTemplate(widget, mediaRequestsWidgetTemplate)
}

type MediaRequest struct {
	Id               int
	Name             string
	Status           int
	Availability     int
	BackdropImageUrl string
	PosterImageUrl   string
	Href             string
	Type             string
	CreatedAt        time.Time
	AirDate          string // TODO: change to time.Time
	RequestedBy      User
}

type mediaRequestsResponse struct {
	Results []MediaRequestData `json:"results"`
}

type MediaRequestData struct {
	Id        int       `json:"id"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	Type      string    `json:"type"`
	Media     struct {
		Id        int       `json:"id"`
		MediaType string    `json:"mediaType"`
		TmdbID    int       `json:"tmdbId"`
		Status    int       `json:"status"`
		CreatedAt time.Time `json:"createdAt"`
	} `json:"media"`
	RequestedBy User `json:"requestedBy"`
}

type User struct {
	Id          int    `json:"id"`
	DisplayName string `json:"displayName"`
	Avatar      string `json:"avatar"`
	Link        string `json:"-"`
}

func fetchMediaRequests(instanceURL string, apiKey string, limit int) ([]MediaRequest, error) {
	if apiKey == "" {
		return nil, errors.New("missing API key")
	}
	requestURL := fmt.Sprintf("%s/api/v1/request?take=%d&sort=added&sortDirection=desc", strings.TrimRight(instanceURL, "/"), limit)

	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("X-Api-Key", apiKey)
	request.Header.Set("accept", "application/json")

	client := defaultHTTPClient
	responseJson, err := decodeJsonFromRequest[mediaRequestsResponse](client, request)
	if err != nil {
		return nil, err
	}

	mediaRequests := make([]MediaRequest, len(responseJson.Results))
	for i, res := range responseJson.Results {
		info, err := fetchItemInformation(instanceURL, apiKey, res.Media.TmdbID, res.Media.MediaType)
		if err != nil {
			return nil, err
		}
		mediaReq := MediaRequest{
			Id:               res.Id,
			Name:             info.Name,
			Status:           res.Status,
			Availability:     res.Media.Status,
			BackdropImageUrl: "https://image.tmdb.org/t/p/original/" + info.BackdropPath,
			PosterImageUrl:   "https://image.tmdb.org/t/p/w600_and_h900_bestv2/" + info.PosterPath,
			Href:             fmt.Sprintf("%s/%s/%d", strings.TrimRight(instanceURL, "/"), res.Type, res.Media.TmdbID),
			Type:             res.Type,
			CreatedAt:        res.CreatedAt,
			AirDate:          info.AirDate,
			RequestedBy: User{
				Id:          res.RequestedBy.Id,
				DisplayName: res.RequestedBy.DisplayName,
				Avatar:      constructAvatarUrl(instanceURL, res.RequestedBy.Avatar),
				Link:        fmt.Sprintf("%s/users/%d", strings.TrimRight(instanceURL, "/"), res.RequestedBy.Id),
			},
		}
		mediaRequests[i] = mediaReq
	}
	return mediaRequests, nil
}

type MediaInfo struct {
	Name         string
	PosterPath   string
	BackdropPath string
	AirDate      string
}

type TvInfo struct {
	Name         string `json:"name"`
	PosterPath   string `json:"posterPath"`
	BackdropPath string `json:"backdropPath"`
	AirDate      string `json:"firstAirDate"`
}

type MovieInfo struct {
	Name         string `json:"name"`
	PosterPath   string `json:"posterPath"`
	BackdropPath string `json:"backdropPath"`
	AirDate      string `json:"releaseDate"`
}

func fetchItemInformation(instanceURL string, apiKey string, id int, mediaType string) (*MediaInfo, error) {
	requestURL := fmt.Sprintf("%s/api/v1/%s/%d", strings.TrimRight(instanceURL, "/"), mediaType, id)

	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("X-Api-Key", apiKey)
	request.Header.Set("accept", "application/json")

	client := defaultHTTPClient
	if mediaType == "tv" {
		series, err := decodeJsonFromRequest[TvInfo](client, request)
		if err != nil {
			return nil, err
		}

		media := MediaInfo{
			Name:         series.Name,
			PosterPath:   series.PosterPath,
			BackdropPath: series.BackdropPath,
			AirDate:      series.AirDate,
		}

		return &media, nil
	}

	movie, err := decodeJsonFromRequest[MovieInfo](client, request)
	if err != nil {
		return nil, err
	}

	media := MediaInfo{
		Name:         movie.Name,
		PosterPath:   movie.PosterPath,
		BackdropPath: movie.BackdropPath,
		AirDate:      movie.AirDate,
	}

	return &media, nil
}

func constructAvatarUrl(instanceURL string, avatar string) string {
	isAbsolute := strings.HasPrefix(avatar, "http://") || strings.HasPrefix(avatar, "https://")

	if isAbsolute {
		return avatar
	}

	return instanceURL + avatar
}
