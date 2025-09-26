package glance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

var torrentsWidgetTemplate = mustParseTemplate("torrents.html", "widget-base.html")

type torrentsWidget struct {
	widgetBase `yaml:",inline"`

	URL           string `yaml:"url"`
	AllowInsecure bool   `yaml:"allow-insecure"`
	Username      string `yaml:"username"`
	Password      string `yaml:"password"`
	Limit         int    `yaml:"limit"`
	CollapseAfter int    `yaml:"collapse-after"`
	Client        string `yaml:"client"`

	SortBy sortableFields[torrent] `yaml:"sort-by"`

	Torrents []torrent `yaml:"-"`

	sessionID string
}

func (widget *torrentsWidget) initialize() error {
	widget.
		withTitle("Torrents").
		withTitleURL(widget.URL).
		withCacheDuration(time.Second * 5)

	if widget.URL == "" {
		return errors.New("URL is required")
	}

	if _, err := url.Parse(widget.URL); err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}

	widget.URL = strings.TrimSuffix(widget.URL, "/")

	if widget.Limit <= 0 {
		widget.Limit = 10
	}

	if widget.CollapseAfter == 0 || widget.CollapseAfter < -1 {
		widget.CollapseAfter = 5
	}

	if widget.Client == "" {
		widget.Client = "qbittorrent"
	}

	if !slices.Contains([]string{"qbittorrent"}, widget.Client) {
		return fmt.Errorf("unsupported client: %s", widget.Client)
	}

	if err := widget.SortBy.Default("downloaded, down-speed:desc, up-speed:desc"); err != nil {
		return err
	}

	if err := widget.SortBy.Fields(map[string]func(a, b torrent) int{
		"name": func(a, b torrent) int {
			return strings.Compare(a.Name, b.Name)
		},
		"progress": func(a, b torrent) int {
			return numCompare(a.Progress, b.Progress)
		},
		"downloaded": func(a, b torrent) int {
			return boolCompare(a.Downloaded, b.Downloaded)
		},
		"down-speed": func(a, b torrent) int {
			return numCompare(a.DownSpeed, b.DownSpeed)
		},
		"up-speed": func(a, b torrent) int {
			return numCompare(a.UpSpeed, b.UpSpeed)
		},
	}); err != nil {
		return err
	}

	return nil
}

func (widget *torrentsWidget) update(ctx context.Context) {
	var torrents []torrent
	var err error

	switch widget.Client {
	case "qbittorrent":
		torrents, err = widget.fetchQbtTorrents()
	default:
		err = fmt.Errorf("unsupported client: %s", widget.Client)
	}

	if !widget.canContinueUpdateAfterHandlingErr(err) {
		return
	}

	widget.SortBy.Apply(torrents)

	if len(torrents) > widget.Limit {
		torrents = torrents[:widget.Limit]
	}

	widget.Torrents = torrents
}

func (widget *torrentsWidget) Render() template.HTML {
	return widget.renderTemplate(widget, torrentsWidgetTemplate)
}

const (
	torrentStatusDownloading = "Downloading"
	torrentStatusDownloaded  = "Downloaded"
	torrentStatusSeeding     = "Seeding"
	torrentStatusPaused      = "Paused"
	torrentStatusStalled     = "Stalled"
	torrentStatusError       = "Error"
	torrentStatusOther       = "Other"
)

// States taken from https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1)#torrent-management
var qbittorrentStates = map[string][2]string{
	// Downloading states
	"downloading": {torrentStatusDownloading, "Torrent is being downloaded and data is being transferred"},
	"metaDL":      {torrentStatusDownloading, "Torrent has just started downloading and is fetching metadata"},
	"forcedDL":    {torrentStatusDownloading, "Torrent is forced to download, ignoring queue limit"},
	"allocating":  {torrentStatusDownloading, "Torrent is allocating disk space for download"},

	// Downloaded/Seeding states
	"checkingUP": {torrentStatusDownloaded, "Torrent has finished downloading and is being checked"},
	"uploading":  {torrentStatusSeeding, "Torrent is being seeded and data is being transferred"},
	"stalledUP":  {torrentStatusSeeding, "Torrent is being seeded, but no connections were made"},
	"forcedUP":   {torrentStatusSeeding, "Torrent is forced to upload, ignoring queue limit"},

	// Stopped/Paused states
	"stoppedDL": {torrentStatusPaused, "Torrent is stopped"},
	"pausedDL":  {torrentStatusPaused, "Torrent is paused and has not finished downloading"},
	"pausedUP":  {torrentStatusPaused, "Torrent is paused and has finished downloading"},
	"queuedDL":  {torrentStatusPaused, "Queuing is enabled and torrent is queued for download"},
	"queuedUP":  {torrentStatusPaused, "Queuing is enabled and torrent is queued for upload"},

	// Stalled states
	"stalledDL": {torrentStatusStalled, "Torrent is being downloaded, but no connections were made"},

	// Error states
	"error":        {torrentStatusError, "An error occurred, applies to paused torrents"},
	"missingFiles": {torrentStatusError, "Torrent data files are missing"},

	// Other states
	"checkingDL":         {torrentStatusOther, "Same as checkingUP, but torrent has not finished downloading"},
	"checkingResumeData": {torrentStatusOther, "Checking resume data on qBittorrent startup"},
	"moving":             {torrentStatusOther, "Torrent is moving to another location"},
	"unknown":            {torrentStatusOther, "Unknown status"},
}

type torrent struct {
	Name               string
	ProgressFormatted  string
	Downloaded         bool
	Progress           float64
	State              string
	StateDescription   string
	UpSpeed            uint64
	DownSpeed          uint64
	DownSpeedFormatted string
	ETAFormatted       string
}

func (widget *torrentsWidget) formatETA(seconds uint64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	} else if seconds < 60*60 {
		return fmt.Sprintf("%dm", seconds/60)
	} else if seconds < 60*60*24 {
		return fmt.Sprintf("%dh", seconds/(60*60))
	} else if seconds < 60*60*24*7 {
		return fmt.Sprintf("%dd", seconds/(60*60*24))
	}

	return fmt.Sprintf("%dw", seconds/(60*60*24*7))
}

func (widget *torrentsWidget) fetchQbtTorrents() ([]torrent, error) {
	if widget.sessionID == "" {
		if err := widget.fetchQbtSessionID(); err != nil {
			return nil, fmt.Errorf("fetching qBittorrent session ID: %v", err)
		}
	}

	torrents, refetchSID, err := widget._fetchQbtTorrents()
	if err != nil {
		if refetchSID {
			if err := widget.fetchQbtSessionID(); err != nil {
				return nil, fmt.Errorf("refetching qBittorrent session ID: %v", err)
			}
			torrents, _, err = widget._fetchQbtTorrents()
			if err != nil {
				return nil, fmt.Errorf("refetching qBittorrent torrents: %v", err)
			}
		} else {
			return nil, fmt.Errorf("fetching qBittorrent torrents: %v", err)
		}
	}

	return torrents, nil
}

func (widget *torrentsWidget) _fetchQbtTorrents() ([]torrent, bool, error) {
	params := url.Values{}
	params.Set("limit", strconv.Itoa(widget.Limit))
	params.Set("sort", "dlspeed")
	params.Set("reverse", "true")

	requestURL := fmt.Sprintf("%s%s?%s", widget.URL, "/api/v2/torrents/info", params.Encode())
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("creating torrents request: %v", err)
	}

	req.Header.Set("Referer", widget.URL)
	req.AddCookie(&http.Cookie{Name: "SID", Value: widget.sessionID})

	client := ternary(widget.AllowInsecure, defaultInsecureHTTPClient, defaultHTTPClient)
	resp, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("torrents request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		// QBittorrent seems to return a 403 if the session ID is invalid or expired.
		refetch := resp.StatusCode == http.StatusForbidden
		return nil, refetch, fmt.Errorf("torrents request failed with status %d: %s", resp.StatusCode, string(body))
	}

	type qbTorrent struct {
		Name      string  `json:"name"`
		Progress  float64 `json:"progress"`
		State     string  `json:"state"`
		DownSpeed uint64  `json:"dlspeed"`
		UpSpeed   uint64  `json:"upspeed"`
		ETA       uint64  `json:"eta"` // in seconds
	}

	var rawTorrents []qbTorrent
	if err := json.Unmarshal(body, &rawTorrents); err != nil {
		return nil, true, fmt.Errorf("decoding torrents response: %v", err)
	}

	torrents := make([]torrent, len(rawTorrents))
	for i, raw := range rawTorrents {
		state := raw.State
		stateDescription := "Unknown state"
		if mappedState, exists := qbittorrentStates[raw.State]; exists {
			state = mappedState[0]
			stateDescription = mappedState[1]
		}

		torrents[i] = torrent{
			Name:              raw.Name,
			Progress:          raw.Progress * 100,
			Downloaded:        raw.Progress >= 1.0,
			ProgressFormatted: fmt.Sprintf("%.1f%%", raw.Progress*100),
			State:             state,
			StateDescription:  stateDescription,
			ETAFormatted:      widget.formatETA(raw.ETA),
			DownSpeed:         raw.DownSpeed,
			UpSpeed:           raw.UpSpeed,
		}

		if raw.DownSpeed > 0 {
			value, unit := formatBytes(raw.DownSpeed)
			torrents[i].DownSpeedFormatted = fmt.Sprintf("%s %s", value, unit)
		}
	}

	return torrents, false, nil
}

func (widget *torrentsWidget) fetchQbtSessionID() error {
	loginData := url.Values{}
	loginData.Set("username", widget.Username)
	loginData.Set("password", widget.Password)

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/api/v2/auth/login", widget.URL),
		strings.NewReader(loginData.Encode()),
	)
	if err != nil {
		return fmt.Errorf("creating login request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", widget.URL)

	client := ternary(widget.AllowInsecure, defaultInsecureHTTPClient, defaultHTTPClient)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("login request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(body))
	}

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		return errors.New("no session cookie received, maybe the username or password is incorrect?")
	}

	for _, cookie := range cookies {
		if cookie.Name == "SID" {
			widget.sessionID = cookie.Value
		}
	}
	if widget.sessionID == "" {
		return errors.New("session ID not found in cookies")
	}

	return nil
}
